package manager_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// TestTagManagerDoubleWrite verifies that SetTags writes to both tags and resource_labels tables
func TestTagManagerDoubleWrite(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenants[0])

	tm := manager.NewTagManager(r)
	keyConfigID := uuid.New()

	// Set tags
	tags := []string{"tag1", "tag2", "tag3"}
	err := tm.SetTags(ctx, keyConfigID, tags)
	assert.NoError(t, err)

	// Verify in tags table (primary)
	retrievedTags, err := tm.GetTags(ctx, keyConfigID)
	assert.NoError(t, err)
	assert.ElementsMatch(t, tags, retrievedTags)

	// Verify in resource_labels table (double-write)
	var resourceLabels []*model.ResourceLabel
	ck := repo.NewCompositeKey().
		Where(repo.ResourceTypeField, model.ResourceTypeKeyConfig).
		Where(repo.ResourceIDField, keyConfigID).
		Where(repo.KeyField, model.SystemTagKey)
	err = r.List(ctx, &model.ResourceLabel{}, &resourceLabels, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
	assert.NoError(t, err)
	assert.Len(t, resourceLabels, 3)

	// Verify tag values match
	syncedValues := make([]string, 0, len(resourceLabels))
	for _, rl := range resourceLabels {
		syncedValues = append(syncedValues, rl.Value)
	}
	assert.ElementsMatch(t, tags, syncedValues)
}

// TestTagManagerDoubleWriteDelete verifies that DeleteTags removes from both tables
func TestTagManagerDoubleWriteDelete(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenants[0])

	tm := manager.NewTagManager(r)
	keyConfigID := uuid.New()

	// Set tags first
	tags := []string{"tag1", "tag2"}
	err := tm.SetTags(ctx, keyConfigID, tags)
	assert.NoError(t, err)

	// Delete tags
	err = tm.DeleteTags(ctx, keyConfigID)
	assert.NoError(t, err)

	// Verify deleted from tags table
	retrievedTags, err := tm.GetTags(ctx, keyConfigID)
	assert.NoError(t, err)
	assert.Empty(t, retrievedTags)

	// Verify deleted from resource_labels table
	var resourceLabels []*model.ResourceLabel
	ck := repo.NewCompositeKey().
		Where(repo.ResourceTypeField, model.ResourceTypeKeyConfig).
		Where(repo.ResourceIDField, keyConfigID).
		Where(repo.KeyField, model.SystemTagKey)
	err = r.List(ctx, &model.ResourceLabel{}, &resourceLabels, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
	assert.NoError(t, err)
	assert.Empty(t, resourceLabels)
}
