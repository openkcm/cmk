package manager_test

import (
	"encoding/json"
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

func SetupTagManager(t *testing.T) (*manager.TagManager,
	*multitenancy.DB, string,
) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	dbRepository := sql.NewRepository(db)
	tagManager := manager.NewTagManager(dbRepository)

	return tagManager, db, tenants[0]
}

// TestGetTagByKeyConfigurationReturnsTags tests when tags are present
func TestGetKeyConfigurationTags(t *testing.T) {
	m, db, tenant := SetupTagManager(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	expectedTagValue := []string{"tag1"}
	bytes, err := json.Marshal(expectedTagValue)
	assert.NoError(t, err)

	id := uuid.New()
	tag := testutils.NewTag(func(t *model.Tag) {
		t.ID = id
		t.Values = bytes
	})

	testutils.CreateTestEntities(ctx, t, r, tag)

	t.Run("Should get tags", func(t *testing.T) {
		tags, err := m.GetTags(ctx, id)
		assert.NoError(t, err)
		assert.Equal(t, expectedTagValue, tags)
	})

	t.Run("Should return empty on non existing tag", func(t *testing.T) {
		tags, err := m.GetTags(ctx, uuid.New())
		assert.NoError(t, err)
		assert.Empty(t, tags)
	})
}

func TestCreateTags(t *testing.T) {
	m, db, tenant := SetupTagManager(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	t.Run("Should set tags", func(t *testing.T) {
		id := uuid.New()
		tags := []string{"tag1", "tag2"}
		err := m.SetTags(ctx, id, tags)
		assert.NoError(t, err)

		tag := &model.Tag{ID: id}
		_, err = r.First(ctx, tag, *repo.NewQuery())
		assert.NoError(t, err)

		res := []string{}
		err = json.Unmarshal(tag.Values, &res)
		assert.NoError(t, err)
		assert.Equal(t, tags, res)

		tags = []string{"tag3", "tag4"}
		err = m.SetTags(ctx, id, tags)
		assert.NoError(t, err)
		_, err = r.First(ctx, tag, *repo.NewQuery())
		assert.NoError(t, err)

		err = json.Unmarshal(tag.Values, &res)
		assert.NoError(t, err)
		assert.Equal(t, tags, res)
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
