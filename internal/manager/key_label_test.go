package manager_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func TestGetKeyLabels(t *testing.T) {
	db, m, tenant := setupTest(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	keyID := uuid.New()
	key := testutils.NewKey(func(k *model.Key) {
		k.ID = keyID
	})
	key2 := testutils.NewKey(func(_ *model.Key) {})
	rl1 := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKey,
		ResourceID:   keyID,
		Key:          "foo",
		Value:        "bar",
	}
	rl2 := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKey,
		ResourceID:   keyID,
		Key:          "region/az",
		Value:        "eu-west-1/a",
	}
	testutils.CreateTestEntities(ctx, t, r, key, key2, rl1, rl2)

	t.Run("Should get key labels", func(t *testing.T) {
		labels, count, err := m.GetKeyLabels(
			testutils.CreateCtxWithTenant(tenant),
			key.ID,
			repo.Pagination{Count: true},
		)
		assert.NoError(t, err)

		assert.Len(t, labels, 2)
		for _, l := range labels {
			assert.Equal(t, keyID, l.ResourceID)
		}
		assert.Equal(t, 2, count)
	})

	t.Run("Should error getting key labels on invalid key", func(t *testing.T) {
		_, _, err := m.GetKeyLabels(
			testutils.CreateCtxWithTenant(tenant),
			uuid.New(),
			repo.Pagination{},
		)
		assert.ErrorIs(t, err, manager.ErrGettingKeyByID)
	})
}

func TestCreateOrUpdateLabel(t *testing.T) {
	db, m, tenant := setupTest(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	key := testutils.NewKey(func(_ *model.Key) {})

	testutils.CreateTestEntities(ctx, t, r, key)

	expected := []*model.KeyLabel{
		{
			BaseLabel: model.BaseLabel{
				Value: "test-1",
				Key:   "test-key",
			},
			CryptoKey: *key,
		},
	}

	t.Run("Should create labels", func(t *testing.T) {
		err := m.CreateOrUpdateLabel(ctx, key.ID, expected)
		assert.NoError(t, err)

		labels, _, _ := m.GetKeyLabels(
			ctx,
			key.ID,
			repo.Pagination{Count: true},
		)

		assert.NoError(t, err)

		for i := range labels {
			assert.Equal(t, expected[i].Key, labels[i].Key)
			assert.Equal(t, expected[i].Value, labels[i].Value)
		}
	})
	t.Run("Should update labels", func(t *testing.T) {
		err := m.CreateOrUpdateLabel(ctx, key.ID, expected)
		assert.NoError(t, err)

		expected[0].Value = "test-2"
		err = m.CreateOrUpdateLabel(ctx, key.ID, expected)
		assert.NoError(t, err)

		labels, count, _ := m.GetKeyLabels(
			ctx,
			key.ID,
			repo.Pagination{Count: true},
		)

		assert.NoError(t, err)
		assert.Equal(t, len(labels), count)
		assert.Equal(t, expected[0].Value, labels[0].Value)
	})
	t.Run("Should error on non existing key", func(t *testing.T) {
		err := m.CreateOrUpdateLabel(ctx, uuid.New(), nil)
		assert.ErrorIs(t, err, manager.ErrGettingKeyByID)
	})
	t.Run("Should error creating key label", func(t *testing.T) {
		key := testutils.NewKey(func(k *model.Key) { k.Name = "test" })
		testutils.CreateTestEntities(ctx, t, r, key)

		forced := testutils.NewDBErrorForced(db, ErrForced)
		forced.WithCreate().Register()
		t.Cleanup(func() {
			forced.Unregister()
		})

		err := m.CreateOrUpdateLabel(ctx, key.ID, expected)
		assert.ErrorIs(t, err, manager.ErrInsertLabel)
	})
}

func TestDeleteKeyLabel(t *testing.T) {
	db, m, tenant := setupTest(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	keyID := uuid.New()
	key := testutils.NewKey(func(k *model.Key) {
		k.ID = keyID
	})

	testutils.CreateTestEntities(ctx, t, r, key)
	err := m.CreateOrUpdateLabel(ctx, key.ID, []*model.KeyLabel{
		{BaseLabel: model.BaseLabel{Key: "foo", Value: "bar"}},
		{BaseLabel: model.BaseLabel{Key: "region/az", Value: "eu-west-1/a"}},
	})
	assert.NoError(t, err)

	t.Run("Should delete label", func(t *testing.T) {
		ok, err := m.DeleteLabel(ctx, key.ID, "foo")
		assert.NoError(t, err)
		assert.True(t, ok)

		labels, _, err := m.GetKeyLabels(
			ctx,
			key.ID,
			repo.Pagination{},
		)
		assert.NoError(t, err)
		assert.Len(t, labels, 1)
	})

	t.Run("Should error on empty label name", func(t *testing.T) {
		_, err := m.DeleteLabel(ctx, key.ID, "")
		assert.ErrorIs(t, err, manager.ErrEmptyInputLabelDB)
	})

	t.Run("Should error invalid key id", func(t *testing.T) {
		_, err := m.DeleteLabel(ctx, uuid.New(), "foo")
		assert.ErrorIs(t, err, manager.ErrGetKeyIDDB)
	})
}

func setupTest(t *testing.T) (*multitenancy.DB, *manager.LabelManager, string) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	r := sql.NewRepository(db)
	resourceLabelManager := manager.NewResourceLabelManager(r)
	labelManager := manager.NewLabelManager(r, resourceLabelManager)

	return db, labelManager, tenants[0]
}
