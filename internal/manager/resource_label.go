package manager

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

// ResourceLabels defines operations for managing labels and tags across different resource types
type ResourceLabels interface {
	// Label operations - manage key-value pairs (excludes system tags)
	GetLabels(
		ctx context.Context,
		resourceType model.ResourceType,
		resourceID uuid.UUID,
		pagination repo.Pagination,
	) ([]*model.ResourceLabel, int, error)
	CreateOrUpdateLabels(
		ctx context.Context,
		resourceType model.ResourceType,
		resourceID uuid.UUID,
		labels []*model.ResourceLabel,
	) error
	DeleteLabel(
		ctx context.Context,
		resourceType model.ResourceType,
		resourceID uuid.UUID,
		labelKey string,
	) (bool, error)
	// DeleteAllLabels removes all labels (excluding tags) for a resource
	DeleteAllLabels(
		ctx context.Context,
		resourceType model.ResourceType,
		resourceID uuid.UUID,
	) error

	// Tag operations - manage tags as special labels with key="system.tag"
	GetTags(ctx context.Context, resourceType model.ResourceType, resourceID uuid.UUID) ([]string, error)
	SetTags(ctx context.Context, resourceType model.ResourceType, resourceID uuid.UUID, tags []string) error
	DeleteTags(ctx context.Context, resourceType model.ResourceType, resourceID uuid.UUID) error
}

// ResourceLabelManager implements the ResourceLabels interface
type ResourceLabelManager struct {
	r repo.Repo
}

// NewResourceLabelManager creates a new ResourceLabelManager
func NewResourceLabelManager(r repo.Repo) *ResourceLabelManager {
	return &ResourceLabelManager{r: r}
}

// GetLabels retrieves all labels for a resource (excluding system tags)
func (m *ResourceLabelManager) GetLabels(
	ctx context.Context,
	resourceType model.ResourceType,
	resourceID uuid.UUID,
	pagination repo.Pagination,
) ([]*model.ResourceLabel, int, error) {
	// Build composite key to filter by resource type and resource ID
	ck := repo.NewCompositeKey().
		Where(repo.ResourceTypeField, resourceType).
		Where(repo.ResourceIDField, resourceID).
		Where(repo.KeyField, model.SystemTagKey, repo.NotEq)

	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))

	labels, count, err := repo.ListAndCount(ctx, m.r, pagination, model.ResourceLabel{}, query)
	if err != nil {
		return nil, 0, errs.Wrap(ErrQueryLabelList, err)
	}

	return labels, count, nil
}

// CreateOrUpdateLabels creates or updates multiple labels for a resource
func (m *ResourceLabelManager) CreateOrUpdateLabels(
	ctx context.Context,
	resourceType model.ResourceType,
	resourceID uuid.UUID,
	labels []*model.ResourceLabel,
) error {
	if len(labels) == 0 {
		return nil
	}

	// Validate that no labels use the reserved system.tag key
	for _, label := range labels {
		if label.Key == model.SystemTagKey {
			return ErrReservedLabelKey
		}
	}

	return m.r.Transaction(ctx, func(ctx context.Context) error {
		for _, label := range labels {
			// Ensure the label has correct resource type and ID
			label.ResourceType = resourceType
			label.ResourceID = resourceID

			if err := m.upsertLabel(ctx, label); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteLabel removes a single label by key
func (m *ResourceLabelManager) DeleteLabel(
	ctx context.Context,
	resourceType model.ResourceType,
	resourceID uuid.UUID,
	labelKey string,
) (bool, error) {
	// Prevent deletion of system tags via DeleteLabel - use DeleteTags instead
	if labelKey == model.SystemTagKey {
		return false, ErrReservedLabelKey
	}

	ck := repo.NewCompositeKey().
		Where(repo.ResourceTypeField, resourceType).
		Where(repo.ResourceIDField, resourceID).
		Where(repo.KeyField, labelKey)

	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))

	deleted, err := m.r.Delete(ctx, &model.ResourceLabel{}, *query)
	if err != nil {
		return false, errs.Wrap(ErrDeleteLabelDB, err)
	}

	return deleted, nil
}

// DeleteAllLabels removes all labels (excluding tags) for a resource
// Used when deleting the parent resource to clean up orphaned labels
func (m *ResourceLabelManager) DeleteAllLabels(
	ctx context.Context,
	resourceType model.ResourceType,
	resourceID uuid.UUID,
) error {
	ck := repo.NewCompositeKey().
		Where(repo.ResourceTypeField, resourceType).
		Where(repo.ResourceIDField, resourceID).
		Where(repo.KeyField, model.SystemTagKey, repo.NotEq) // Exclude system tags

	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))

	_, err := m.r.Delete(ctx, &model.ResourceLabel{}, *query)
	if err != nil {
		return errs.Wrap(ErrDeleteLabelDB, err)
	}

	return nil
}

// GetTags retrieves all tag values for a resource
func (m *ResourceLabelManager) GetTags(
	ctx context.Context,
	resourceType model.ResourceType,
	resourceID uuid.UUID,
) ([]string, error) {
	// Query all labels with key="system.tag"
	ck := repo.NewCompositeKey().
		Where(repo.ResourceTypeField, resourceType).
		Where(repo.ResourceIDField, resourceID).
		Where(repo.KeyField, model.SystemTagKey)

	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))

	labels := []*model.ResourceLabel{}
	err := m.r.List(ctx, &model.ResourceLabel{}, &labels, *query)
	if err != nil {
		return nil, errs.Wrap(ErrGetTags, err)
	}

	// Extract tag values
	tags := make([]string, 0, len(labels))
	for _, label := range labels {
		tags = append(tags, label.Value)
	}

	return tags, nil
}

// SetTags replaces all tags for a resource
func (m *ResourceLabelManager) SetTags(
	ctx context.Context,
	resourceType model.ResourceType,
	resourceID uuid.UUID,
	tags []string,
) error {
	// Special case: single empty string triggers deletion (backwards compatibility)
	if len(tags) == 1 && tags[0] == "" {
		return m.DeleteTags(ctx, resourceType, resourceID)
	}

	return m.r.Transaction(ctx, func(ctx context.Context) error {
		// Delete existing tags first
		ck := repo.NewCompositeKey().
			Where(repo.ResourceTypeField, resourceType).
			Where(repo.ResourceIDField, resourceID).
			Where(repo.KeyField, model.SystemTagKey)

		query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))
		_, err := m.r.Delete(ctx, &model.ResourceLabel{}, *query)
		if err != nil {
			return errs.Wrap(ErrDeletingTags, err)
		}

		// Deduplicate and filter empty tags
		uniqueTags := make(map[string]struct{})
		for _, tag := range tags {
			if tag != "" {
				uniqueTags[tag] = struct{}{}
			}
		}

		// Create new tags
		for tag := range uniqueTags {
			label := &model.ResourceLabel{
				ID:           uuid.New(),
				ResourceType: resourceType,
				ResourceID:   resourceID,
				Key:          model.SystemTagKey,
				Value:        tag,
			}

			err = m.r.Create(ctx, label)
			if err != nil {
				return errs.Wrap(ErrCreateTag, err)
			}
		}

		return nil
	})
}

// DeleteTags removes all tags for a resource
func (m *ResourceLabelManager) DeleteTags(
	ctx context.Context,
	resourceType model.ResourceType,
	resourceID uuid.UUID,
) error {
	ck := repo.NewCompositeKey().
		Where(repo.ResourceTypeField, resourceType).
		Where(repo.ResourceIDField, resourceID).
		Where(repo.KeyField, model.SystemTagKey)

	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))

	_, err := m.r.Delete(ctx, &model.ResourceLabel{}, *query)
	if err != nil {
		return errs.Wrap(ErrDeletingTags, err)
	}

	return nil
}

// upsertLabel creates or updates a single label atomically with bounded retries
func (m *ResourceLabelManager) upsertLabel(ctx context.Context, label *model.ResourceLabel) error {
	const maxRetries = 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		// First check if the label already exists
		existing, found, findErr := m.findLabelByKey(ctx, label.ResourceType, label.ResourceID, label.Key)
		if findErr != nil {
			return errs.Wrap(ErrFetchLabel, findErr)
		}

		if found {
			// Label exists - check if update is needed
			if existing.Value == label.Value {
				// No update needed - value is already correct
				return nil
			}

			// Update the existing label
			existing.Value = label.Value
			_, err := m.r.Patch(ctx, existing, *repo.NewQuery().UpdateAll(true))
			if err != nil {
				return errs.Wrap(ErrUpdateLabelDB, err)
			}
			return nil
		}

		// Label doesn't exist - try to create it
		label.ID = uuid.New()
		err := m.r.Create(ctx, label)
		if err != nil {
			// Check if it's a unique constraint violation (race condition)
			if errors.Is(err, repo.ErrUniqueConstraint) {
				// Another transaction created it between our check and insert
				// Retry the whole operation (loop continues)
				continue
			}
			return errs.Wrap(ErrInsertLabel, err)
		}

		// Successfully created
		return nil
	}

	// Exceeded max retries - make a final attempt to update the existing row
	// that won the race to ensure the desired value is set
	existing, found, findErr := m.findLabelByKey(ctx, label.ResourceType, label.ResourceID, label.Key)
	if findErr != nil {
		return errs.Wrap(ErrFetchLabel, findErr)
	}

	if found && existing.Value != label.Value {
		existing.Value = label.Value
		_, err := m.r.Patch(ctx, existing, *repo.NewQuery().UpdateAll(true))
		if err != nil {
			return errs.Wrap(ErrUpdateLabelDB, err)
		}
		return nil
	}

	// If we got here, either the label exists with the correct value or something is wrong
	if found {
		return nil
	}

	// Should not reach here under normal circumstances
	return errs.Wrap(ErrInsertLabel, errors.New("exceeded retry limit and failed to create or update label"))
}

// findLabelByKey finds a label with matching resource type, ID, and key
func (m *ResourceLabelManager) findLabelByKey(
	ctx context.Context,
	resourceType model.ResourceType,
	resourceID uuid.UUID,
	key string,
) (*model.ResourceLabel, bool, error) {
	ck := repo.NewCompositeKey().
		Where(repo.ResourceTypeField, resourceType).
		Where(repo.ResourceIDField, resourceID).
		Where(repo.KeyField, key)

	existing := &model.ResourceLabel{}
	found, err := m.r.First(ctx, existing, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, false, err
	}

	return existing, found, nil
}
