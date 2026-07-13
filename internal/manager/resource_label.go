package manager

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

// ResourceLabels defines operations for managing labels and tags across different resource types
type ResourceLabels interface {
	// Label operations - manage key-value pairs (excludes system tags)
	GetLabels(ctx context.Context, resourceType model.ResourceType, resourceID uuid.UUID, pagination repo.Pagination) ([]*model.ResourceLabel, int, error)
	CreateOrUpdateLabels(ctx context.Context, resourceType model.ResourceType, resourceID uuid.UUID, labels []*model.ResourceLabel) error
	DeleteLabel(ctx context.Context, resourceType model.ResourceType, resourceID uuid.UUID, labelKey string) (bool, error)

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

	return m.r.Transaction(ctx, func(ctx context.Context) error {
		for _, label := range labels {
			// Ensure the label has correct resource type and ID
			label.ResourceType = resourceType
			label.ResourceID = resourceID

			// Check if label already exists
			ck := repo.NewCompositeKey().
				Where(repo.ResourceTypeField, resourceType).
				Where(repo.ResourceIDField, resourceID).
				Where(repo.KeyField, label.Key).
				Where(repo.ValueField, label.Value)

			existing := &model.ResourceLabel{}
			found, err := m.r.First(ctx, existing, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
			if err != nil && !errors.Is(err, repo.ErrNotFound) {
				return errs.Wrap(ErrFetchLabel, err)
			}

			if found {
				// Label already exists with same value, skip
				continue
			}

			// Check if a label with same key but different value exists
			ck = repo.NewCompositeKey().
				Where(repo.ResourceTypeField, resourceType).
				Where(repo.ResourceIDField, resourceID).
				Where(repo.KeyField, label.Key)

			found, err = m.r.First(ctx, existing, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
			if err != nil && !errors.Is(err, repo.ErrNotFound) {
				return errs.Wrap(ErrFetchLabel, err)
			}

			if found {
				// Update existing label with new value
				existing.Value = label.Value

				_, err = m.r.Patch(ctx, existing, *repo.NewQuery().UpdateAll(true))
				if err != nil {
					return errs.Wrap(ErrUpdateLabelDB, err)
				}
			} else {
				// Create new label
				label.ID = uuid.New()
				err = m.r.Create(ctx, label)
				if err != nil {
					return errs.Wrap(ErrInsertLabel, err)
				}
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

		// Create new tags
		for _, tag := range tags {
			if tag == "" {
				continue // Skip empty tags
			}

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

// Helper to marshal tags to JSON (for backwards compatibility if needed)
func marshalTags(tags []string) (json.RawMessage, error) {
	bytes, err := json.Marshal(tags)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(bytes), nil
}
