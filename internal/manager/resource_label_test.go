package manager_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func setupResourceLabelManager(t *testing.T) (*manager.ResourceLabelManager, *multitenancy.DB, string) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	dbRepository := sql.NewRepository(db)
	resourceLabelManager := manager.NewResourceLabelManager(dbRepository)

	return resourceLabelManager, db, tenants[0]
}

// TestGetLabels tests retrieving labels for a resource
func TestGetLabels(t *testing.T) {
	m, db, tenant := setupResourceLabelManager(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	resourceID := uuid.New()

	// Create test labels
	label1 := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKey,
		ResourceID:   resourceID,
		Key:          "environment",
		Value:        "production",
	}
	label2 := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKey,
		ResourceID:   resourceID,
		Key:          "region",
		Value:        "eu-west-1",
	}
	// Create a system tag (should be excluded from labels)
	tag := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKey,
		ResourceID:   resourceID,
		Key:          model.SystemTagKey,
		Value:        "test-tag",
	}
	// Create label for different resource (should not be returned)
	otherLabel := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKey,
		ResourceID:   uuid.New(),
		Key:          "other",
		Value:        "other",
	}

	testutils.CreateTestEntities(ctx, t, r, label1, label2, tag, otherLabel)

	t.Run("Should get labels excluding system tags", func(t *testing.T) {
		labels, count, err := m.GetLabels(ctx, model.ResourceTypeKey, resourceID, repo.Pagination{Count: true})
		require.NoError(t, err)
		assert.Len(t, labels, 2)
		assert.Equal(t, 2, count)

		// Verify labels are returned
		foundEnv := false
		foundRegion := false
		for _, label := range labels {
			assert.Equal(t, resourceID, label.ResourceID)
			assert.NotEqual(t, model.SystemTagKey, label.Key)
			if label.Key == "environment" {
				foundEnv = true
				assert.Equal(t, "production", label.Value)
			}
			if label.Key == "region" {
				foundRegion = true
				assert.Equal(t, "eu-west-1", label.Value)
			}
		}
		assert.True(t, foundEnv)
		assert.True(t, foundRegion)
	})

	t.Run("Should return empty for non-existing resource", func(t *testing.T) {
		labels, count, err := m.GetLabels(ctx, model.ResourceTypeKey, uuid.New(), repo.Pagination{Count: true})
		require.NoError(t, err)
		assert.Empty(t, labels)
		assert.Equal(t, 0, count)
	})

	t.Run("Should support pagination", func(t *testing.T) {
		labels, _, err := m.GetLabels(ctx, model.ResourceTypeKey, resourceID, repo.Pagination{
			Top:  1,
			Skip: 0,
		})
		require.NoError(t, err)
		assert.Len(t, labels, 1)
	})

	t.Run("Should filter by resource type", func(t *testing.T) {
		// Query with wrong resource type
		labels, _, err := m.GetLabels(ctx, model.ResourceTypeKeyConfig, resourceID, repo.Pagination{})
		require.NoError(t, err)
		assert.Empty(t, labels)
	})
}

// TestCreateOrUpdateLabels tests creating and updating labels
func TestCreateOrUpdateLabels(t *testing.T) {
	m, _, tenant := setupResourceLabelManager(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

	resourceID := uuid.New()

	t.Run("Should create new labels", func(t *testing.T) {
		labels := []*model.ResourceLabel{
			{
				Key:   "environment",
				Value: "production",
			},
			{
				Key:   "region",
				Value: "eu-west-1",
			},
		}

		err := m.CreateOrUpdateLabels(ctx, model.ResourceTypeKey, resourceID, labels)
		require.NoError(t, err)

		// Verify labels were created
		retrieved, _, err := m.GetLabels(ctx, model.ResourceTypeKey, resourceID, repo.Pagination{})
		require.NoError(t, err)
		assert.Len(t, retrieved, 2)
	})

	t.Run("Should update existing label value", func(t *testing.T) {
		// Update environment label
		labels := []*model.ResourceLabel{
			{
				Key:   "environment",
				Value: "staging",
			},
		}

		err := m.CreateOrUpdateLabels(ctx, model.ResourceTypeKey, resourceID, labels)
		require.NoError(t, err)

		// Verify label was updated
		retrieved, _, err := m.GetLabels(ctx, model.ResourceTypeKey, resourceID, repo.Pagination{})
		require.NoError(t, err)

		found := false
		for _, label := range retrieved {
			if label.Key == "environment" {
				found = true
				assert.Equal(t, "staging", label.Value)
			}
		}
		assert.True(t, found)
	})

	t.Run("Should not create duplicate labels", func(t *testing.T) {
		labels := []*model.ResourceLabel{
			{
				Key:   "region",
				Value: "eu-west-1",
			},
		}

		err := m.CreateOrUpdateLabels(ctx, model.ResourceTypeKey, resourceID, labels)
		require.NoError(t, err)

		// Verify no duplicates
		retrieved, count, err := m.GetLabels(ctx, model.ResourceTypeKey, resourceID, repo.Pagination{Count: true})
		require.NoError(t, err)
		assert.Equal(t, 2, count)
		assert.Len(t, retrieved, 2)
	})

	t.Run("Should handle empty label slice", func(t *testing.T) {
		err := m.CreateOrUpdateLabels(ctx, model.ResourceTypeKey, uuid.New(), []*model.ResourceLabel{})
		require.NoError(t, err)
	})

	t.Run("Should rollback on error in transaction", func(t *testing.T) {
		newResourceID := uuid.New()

		// First create a valid label
		validLabel := []*model.ResourceLabel{
			{
				Key:   "test",
				Value: "value",
			},
		}
		err := m.CreateOrUpdateLabels(ctx, model.ResourceTypeKey, newResourceID, validLabel)
		require.NoError(t, err)

		// Verify it exists
		labels, _, err := m.GetLabels(ctx, model.ResourceTypeKey, newResourceID, repo.Pagination{})
		require.NoError(t, err)
		assert.Len(t, labels, 1)
	})
}

// TestDeleteLabel tests deleting a single label
func TestDeleteLabel(t *testing.T) {
	m, db, tenant := setupResourceLabelManager(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	resourceID := uuid.New()

	// Create test labels
	label1 := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKey,
		ResourceID:   resourceID,
		Key:          "environment",
		Value:        "production",
	}
	label2 := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKey,
		ResourceID:   resourceID,
		Key:          "region",
		Value:        "eu-west-1",
	}

	testutils.CreateTestEntities(ctx, t, r, label1, label2)

	t.Run("Should delete label by key", func(t *testing.T) {
		deleted, err := m.DeleteLabel(ctx, model.ResourceTypeKey, resourceID, "environment")
		require.NoError(t, err)
		assert.True(t, deleted)

		// Verify label was deleted
		labels, _, err := m.GetLabels(ctx, model.ResourceTypeKey, resourceID, repo.Pagination{})
		require.NoError(t, err)
		assert.Len(t, labels, 1)
		assert.Equal(t, "region", labels[0].Key)
	})

	t.Run("Should return false for non-existing label", func(t *testing.T) {
		deleted, err := m.DeleteLabel(ctx, model.ResourceTypeKey, resourceID, "non-existing")
		require.NoError(t, err)
		assert.False(t, deleted)
	})

	t.Run("Should not delete labels from different resource", func(t *testing.T) {
		deleted, err := m.DeleteLabel(ctx, model.ResourceTypeKey, uuid.New(), "region")
		require.NoError(t, err)
		assert.False(t, deleted)

		// Verify original label still exists
		labels, _, err := m.GetLabels(ctx, model.ResourceTypeKey, resourceID, repo.Pagination{})
		require.NoError(t, err)
		assert.Len(t, labels, 1)
	})
}

// TestGetTags tests retrieving tags for a resource
func TestGetTags(t *testing.T) {
	m, db, tenant := setupResourceLabelManager(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	resourceID := uuid.New()

	// Create test tags
	tag1 := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKeyConfig,
		ResourceID:   resourceID,
		Key:          model.SystemTagKey,
		Value:        "EU",
	}
	tag2 := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKeyConfig,
		ResourceID:   resourceID,
		Key:          model.SystemTagKey,
		Value:        "TEST",
	}
	// Create regular label (should not be returned as tag)
	label := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKeyConfig,
		ResourceID:   resourceID,
		Key:          "environment",
		Value:        "production",
	}

	testutils.CreateTestEntities(ctx, t, r, tag1, tag2, label)

	t.Run("Should get tags as string array", func(t *testing.T) {
		tags, err := m.GetTags(ctx, model.ResourceTypeKeyConfig, resourceID)
		require.NoError(t, err)
		assert.Len(t, tags, 2)
		assert.Contains(t, tags, "EU")
		assert.Contains(t, tags, "TEST")
	})

	t.Run("Should return empty array for resource without tags", func(t *testing.T) {
		tags, err := m.GetTags(ctx, model.ResourceTypeKeyConfig, uuid.New())
		require.NoError(t, err)
		assert.Empty(t, tags)
	})

	t.Run("Should filter by resource type", func(t *testing.T) {
		// Query with wrong resource type
		tags, err := m.GetTags(ctx, model.ResourceTypeKey, resourceID)
		require.NoError(t, err)
		assert.Empty(t, tags)
	})
}

// TestSetTags tests replacing all tags for a resource
func TestSetTags(t *testing.T) {
	m, db, tenant := setupResourceLabelManager(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	resourceID := uuid.New()

	t.Run("Should set tags for new resource", func(t *testing.T) {
		tags := []string{"EU", "TEST", "PRODUCTION"}
		err := m.SetTags(ctx, model.ResourceTypeKeyConfig, resourceID, tags)
		require.NoError(t, err)

		// Verify tags were created
		retrieved, err := m.GetTags(ctx, model.ResourceTypeKeyConfig, resourceID)
		require.NoError(t, err)
		assert.ElementsMatch(t, tags, retrieved)
	})

	t.Run("Should replace existing tags", func(t *testing.T) {
		newTags := []string{"US", "STAGING"}
		err := m.SetTags(ctx, model.ResourceTypeKeyConfig, resourceID, newTags)
		require.NoError(t, err)

		// Verify old tags were replaced
		retrieved, err := m.GetTags(ctx, model.ResourceTypeKeyConfig, resourceID)
		require.NoError(t, err)
		assert.ElementsMatch(t, newTags, retrieved)
		assert.NotContains(t, retrieved, "EU")
		assert.NotContains(t, retrieved, "TEST")
	})

	t.Run("Should skip empty tag values", func(t *testing.T) {
		newResourceID := uuid.New()
		tags := []string{"VALID", "", "ALSO_VALID"}
		err := m.SetTags(ctx, model.ResourceTypeKeyConfig, newResourceID, tags)
		require.NoError(t, err)

		retrieved, err := m.GetTags(ctx, model.ResourceTypeKeyConfig, newResourceID)
		require.NoError(t, err)
		assert.Len(t, retrieved, 2)
		assert.Contains(t, retrieved, "VALID")
		assert.Contains(t, retrieved, "ALSO_VALID")
	})

	t.Run("Should delete tags when single empty string is provided", func(t *testing.T) {
		// First set some tags
		newResourceID := uuid.New()
		err := m.SetTags(ctx, model.ResourceTypeKeyConfig, newResourceID, []string{"TAG1", "TAG2"})
		require.NoError(t, err)

		// Delete with single empty string (backwards compatibility)
		err = m.SetTags(ctx, model.ResourceTypeKeyConfig, newResourceID, []string{""})
		require.NoError(t, err)

		// Verify tags were deleted
		retrieved, err := m.GetTags(ctx, model.ResourceTypeKeyConfig, newResourceID)
		require.NoError(t, err)
		assert.Empty(t, retrieved)
	})

	t.Run("Should not affect regular labels", func(t *testing.T) {
		newResourceID := uuid.New()

		// Create a regular label
		label := &model.ResourceLabel{
			ID:           uuid.New(),
			ResourceType: model.ResourceTypeKeyConfig,
			ResourceID:   newResourceID,
			Key:          "environment",
			Value:        "production",
		}
		testutils.CreateTestEntities(ctx, t, r, label)

		// Set tags
		err := m.SetTags(ctx, model.ResourceTypeKeyConfig, newResourceID, []string{"TAG1"})
		require.NoError(t, err)

		// Verify label still exists
		labels, _, err := m.GetLabels(ctx, model.ResourceTypeKeyConfig, newResourceID, repo.Pagination{})
		require.NoError(t, err)
		assert.Len(t, labels, 1)
		assert.Equal(t, "environment", labels[0].Key)
	})
}

// TestDeleteTags tests removing all tags for a resource
func TestDeleteTags(t *testing.T) {
	m, db, tenant := setupResourceLabelManager(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	resourceID := uuid.New()

	// Create test tags
	tag1 := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKeyConfig,
		ResourceID:   resourceID,
		Key:          model.SystemTagKey,
		Value:        "EU",
	}
	tag2 := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKeyConfig,
		ResourceID:   resourceID,
		Key:          model.SystemTagKey,
		Value:        "TEST",
	}
	// Create regular label (should not be deleted)
	label := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKeyConfig,
		ResourceID:   resourceID,
		Key:          "environment",
		Value:        "production",
	}

	testutils.CreateTestEntities(ctx, t, r, tag1, tag2, label)

	t.Run("Should delete all tags", func(t *testing.T) {
		err := m.DeleteTags(ctx, model.ResourceTypeKeyConfig, resourceID)
		require.NoError(t, err)

		// Verify tags were deleted
		tags, err := m.GetTags(ctx, model.ResourceTypeKeyConfig, resourceID)
		require.NoError(t, err)
		assert.Empty(t, tags)
	})

	t.Run("Should not delete regular labels", func(t *testing.T) {
		// Verify label still exists
		labels, _, err := m.GetLabels(ctx, model.ResourceTypeKeyConfig, resourceID, repo.Pagination{})
		require.NoError(t, err)
		assert.Len(t, labels, 1)
		assert.Equal(t, "environment", labels[0].Key)
	})

	t.Run("Should not error when deleting non-existing tags", func(t *testing.T) {
		err := m.DeleteTags(ctx, model.ResourceTypeKeyConfig, uuid.New())
		require.NoError(t, err)
	})
}

// TestResourceTypeIsolation tests that operations on one resource type don't affect another
func TestResourceTypeIsolation(t *testing.T) {
	m, db, tenant := setupResourceLabelManager(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	resourceID := uuid.New()

	// Create labels for KEY resource type
	keyLabel := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKey,
		ResourceID:   resourceID,
		Key:          "key-label",
		Value:        "key-value",
	}

	// Create labels for KEY_CONFIG resource type
	keyConfigLabel := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKeyConfig,
		ResourceID:   resourceID,
		Key:          "config-label",
		Value:        "config-value",
	}

	testutils.CreateTestEntities(ctx, t, r, keyLabel, keyConfigLabel)

	t.Run("Should isolate labels by resource type", func(t *testing.T) {
		// Get KEY labels
		keyLabels, _, err := m.GetLabels(ctx, model.ResourceTypeKey, resourceID, repo.Pagination{})
		require.NoError(t, err)
		assert.Len(t, keyLabels, 1)
		assert.Equal(t, "key-label", keyLabels[0].Key)

		// Get KEY_CONFIG labels
		configLabels, _, err := m.GetLabels(ctx, model.ResourceTypeKeyConfig, resourceID, repo.Pagination{})
		require.NoError(t, err)
		assert.Len(t, configLabels, 1)
		assert.Equal(t, "config-label", configLabels[0].Key)
	})

	t.Run("Should isolate deletes by resource type", func(t *testing.T) {
		// Delete KEY label
		deleted, err := m.DeleteLabel(ctx, model.ResourceTypeKey, resourceID, "key-label")
		require.NoError(t, err)
		assert.True(t, deleted)

		// Verify KEY_CONFIG label still exists
		configLabels, _, err := m.GetLabels(ctx, model.ResourceTypeKeyConfig, resourceID, repo.Pagination{})
		require.NoError(t, err)
		assert.Len(t, configLabels, 1)
	})
}
