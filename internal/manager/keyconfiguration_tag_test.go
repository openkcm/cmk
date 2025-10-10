package manager_test

import (
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

func SetupKeyConfigurationTagManager(t *testing.T) (*manager.KeyConfigurationTagManager,
	*multitenancy.DB, string,
) {
	t.Helper()

	db, tenants := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{&model.KeyConfiguration{}, &model.KeyConfigurationTag{}},
	})

	dbRepository := sql.NewRepository(db)
	tagManager := manager.NewKeyConfigurationTagManager(dbRepository)

	return tagManager, db, tenants[0]
}

// TestGetTagByKeyConfigurationReturnsTags tests when tags are present
func TestGetKeyConfigurationTags(t *testing.T) {
	m, db, tenant := SetupKeyConfigurationTagManager(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	keyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
		k.Tags = []model.KeyConfigurationTag{
			{
				BaseTag: model.BaseTag{
					ID: uuid.New(), Value: "tag1",
				},
			},
			{
				BaseTag: model.BaseTag{
					ID: uuid.New(), Value: "tag2",
				},
			},
		}
	})

	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	t.Run("Should get tags", func(t *testing.T) {
		tags, err := m.GetTagByKeyConfiguration(ctx, keyConfig.ID)
		assert.NoError(t, err)
		assert.Equal(t, keyConfig.Tags, tags)
	})

	t.Run("Should error on non existent key config", func(t *testing.T) {
		_, err := m.GetTagByKeyConfiguration(ctx, uuid.New())
		assert.ErrorIs(t, err, manager.ErrGetKeyConfig)
	})

	t.Run("Should return empty on non existent key id", func(t *testing.T) {
		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		testutils.CreateTestEntities(ctx, t, r, keyConfig)
		tags, err := m.GetTagByKeyConfiguration(ctx, keyConfig.ID)
		assert.NoError(t, err)
		assert.Empty(t, tags)
	})
}

func TestCreateKeyConfigurationTags(t *testing.T) {
	tagRepo, db, tenant := SetupKeyConfigurationTagManager(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	t.Run("Should create tags", func(t *testing.T) {
		keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
		testutils.CreateTestEntities(ctx, t, r, keyConfig)

		tags := []*model.KeyConfigurationTag{
			{
				BaseTag: model.BaseTag{
					ID: uuid.New(), Value: "tag1",
				},
			},
			{
				BaseTag: model.BaseTag{
					ID: uuid.New(), Value: "tag2",
				},
			},
		}

		err := tagRepo.CreateTagsByKeyConfiguration(ctx, keyConfig.ID, tags)
		assert.NoError(t, err)

		result := &model.KeyConfiguration{ID: keyConfig.ID}

		_, err = r.First(ctx, result, *repo.NewQuery().Preload(repo.Preload{"Tags"}))
		assert.NoError(t, err)

		expectedTags := ptr.PointerArrayToValueArray[model.KeyConfigurationTag](tags)
		assert.Equal(t, expectedTags, result.Tags)
		assert.Equal(t, expectedTags, result.Tags)
	})

	t.Run("Should replace tags", func(t *testing.T) {
		keyConfig := testutils.NewKeyConfig(func(k *model.KeyConfiguration) {
			k.Tags = []model.KeyConfigurationTag{
				{
					BaseTag: model.BaseTag{
						ID: uuid.New(), Value: "tag10",
					},
				},
				{
					BaseTag: model.BaseTag{
						ID: uuid.New(), Value: "tag20",
					},
				},
			}
		})

		testutils.CreateTestEntities(ctx, t, r, keyConfig)

		tags := []*model.KeyConfigurationTag{
			{
				BaseTag: model.BaseTag{
					ID: uuid.New(), Value: "tag3",
				},
			},
			{
				BaseTag: model.BaseTag{
					ID: uuid.New(), Value: "tag4",
				},
			},
		}

		err := tagRepo.CreateTagsByKeyConfiguration(ctx, keyConfig.ID, tags)
		assert.NoError(t, err)

		result := &model.KeyConfiguration{ID: keyConfig.ID}

		_, err = r.First(ctx, result, *repo.NewQuery().Preload(repo.Preload{"Tags"}))
		assert.NoError(t, err)

		expectedTags := ptr.PointerArrayToValueArray[model.KeyConfigurationTag](tags)
		assert.Equal(t, expectedTags, result.Tags)
	})
}

func TestCreateTagsError(t *testing.T) {
	tagRepo, db, tenant := SetupKeyConfigurationTagManager(t)

	t.Run("Should error setting tags on non existing config", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)
		forced.Register()
		t.Cleanup(func() {
			forced.Unregister()
		})

		tags := []*model.KeyConfigurationTag{
			{
				BaseTag: model.BaseTag{
					ID: uuid.New(), Value: "tag3",
				},
			},
			{
				BaseTag: model.BaseTag{
					ID: uuid.New(), Value: "tag4",
				},
			},
		}

		err := tagRepo.CreateTagsByKeyConfiguration(testutils.CreateCtxWithTenant(tenant), uuid.New(), tags)
		assert.ErrorIs(t, err, manager.ErrCreateTag)
	})
}
