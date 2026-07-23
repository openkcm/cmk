package manager_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func SetupTagManager(t *testing.T) (*manager.TagManager,
	*multitenancy.DB, string,
) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	dbRepository := sql.NewRepository(db)
	resourceLabelManager := manager.NewResourceLabelManager(dbRepository)
	tagManager := manager.NewTagManager(resourceLabelManager)

	return tagManager, db, tenants[0]
}

// TestGetTagByKeyConfigurationReturnsTags tests when tags are present
func TestGetKeyConfigurationTags(t *testing.T) {
	m, db, tenant := SetupTagManager(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	id := uuid.New()

	// Create tags using the new resource_labels format
	tag1 := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKeyConfig,
		ResourceID:   id,
		Key:          model.SystemTagKey,
		Value:        "tag1",
	}

	testutils.CreateTestEntities(ctx, t, r, tag1)

	t.Run("Should get tags", func(t *testing.T) {
		tags, err := m.GetTags(ctx, id)
		assert.NoError(t, err)
		assert.Equal(t, []string{"tag1"}, tags)
	})

	t.Run("Should return empty on non existing tag", func(t *testing.T) {
		tags, err := m.GetTags(ctx, uuid.New())
		assert.NoError(t, err)
		assert.Empty(t, tags)
	})
}

func TestCreateTags(t *testing.T) {
	m, _, tenant := SetupTagManager(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

	t.Run("Should set tags", func(t *testing.T) {
		id := uuid.New()
		tags := []string{"tag1", "tag2"}
		err := m.SetTags(ctx, id, tags)
		assert.NoError(t, err)

		// Verify tags were created in resource_labels table
		retrievedTags, err := m.GetTags(ctx, id)
		assert.NoError(t, err)
		assert.ElementsMatch(t, tags, retrievedTags)

		// Update tags
		tags = []string{"tag3", "tag4"}
		err = m.SetTags(ctx, id, tags)
		assert.NoError(t, err)

		retrievedTags, err = m.GetTags(ctx, id)
		assert.NoError(t, err)
		assert.ElementsMatch(t, tags, retrievedTags)
	})

	t.Run("Should not write empty tag", func(t *testing.T) {
		id := uuid.New()
		err := m.SetTags(ctx, id, []string{""})
		assert.NoError(t, err)

		tags, err := m.GetTags(ctx, id)
		assert.NoError(t, err)
		assert.Empty(t, tags)
	})
}
