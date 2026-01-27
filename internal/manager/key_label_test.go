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
		k.KeyLabels = []model.KeyLabel{
			*testutils.NewKeyLabel(func(l *model.KeyLabel) {
				l.ResourceID = keyID
			}),
			*testutils.NewKeyLabel(func(l *model.KeyLabel) {
				l.ResourceID = keyID
			}),
		}
	})
	key2 := testutils.NewKey(func(_ *model.Key) {})
	testutils.CreateTestEntities(ctx, t, r, key, key2)

	t.Run("Should get key labels", func(t *testing.T) {
		labels, count, err := m.GetKeyLabels(
			testutils.CreateCtxWithTenant(tenant),
			key.ID,
			0,
			repo.DefaultLimit,
		)
		assert.NoError(t, err)

		for i := range labels {
			assert.Equal(t, key.KeyLabels[i].ResourceID, labels[i].ResourceID)
		}

		assert.Equal(t, len(key.KeyLabels), count)
	})

	t.Run("Should error getting key labels on invalid key", func(t *testing.T) {
		_, _, err := m.GetKeyLabels(
			testutils.CreateCtxWithTenant(tenant),
			uuid.New(),
			0,
			repo.DefaultLimit,
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
				ID:    uuid.New(),
				Value: "test-1",
				Key:   key.ID.String(),
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
			0,
			repo.DefaultLimit,
		)

		assert.NoError(t, err)

		for i := range labels {
			assert.Equal(t, expected[i].BaseLabel, labels[i].BaseLabel)
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
			0,
			repo.DefaultLimit,
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
		k.KeyLabels = []model.KeyLabel{
			*testutils.NewKeyLabel(func(l *model.KeyLabel) {
				l.ResourceID = keyID
			}),
			*testutils.NewKeyLabel(func(l *model.KeyLabel) {
				l.ResourceID = keyID
			}),
		}
	})

	testutils.CreateTestEntities(ctx, t, r, key)
	t.Run("Should delete label", func(t *testing.T) {
		ok, err := m.DeleteLabel(ctx, key.ID, key.KeyLabels[0].Key)
		assert.NoError(t, err)
		assert.True(t, ok)

		labels, _, err := m.GetKeyLabels(
			ctx,
			key.ID,
			0,
			repo.DefaultLimit,
		)
		assert.NoError(t, err)
		assert.Len(t, labels, 1)
	})

	t.Run("Should error on empty label name", func(t *testing.T) {
		_, err := m.DeleteLabel(ctx, key.ID, "")
		assert.ErrorIs(t, err, manager.ErrEmptyInputLabelDB)
	})

	t.Run("Should error invalid key id", func(t *testing.T) {
		_, err := m.DeleteLabel(ctx, uuid.New(), key.KeyLabels[0].Key)
		assert.ErrorIs(t, err, manager.ErrGetKeyIDDB)
	})
}

func setupTest(t *testing.T) (*multitenancy.DB, *manager.LabelManager, string) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	r := sql.NewRepository(db)
	labelManager := manager.NewLabelManager(r)

	return db, labelManager, tenants[0]
}
