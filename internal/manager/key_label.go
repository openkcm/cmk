package manager

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

// Label interface for managing labels on keys
type Label interface {
	GetKeyLabels(
		ctx context.Context,
		keyID uuid.UUID,
		pagination repo.Pagination,
	) ([]*model.KeyLabel, int, error)
	CreateOrUpdateLabel(
		ctx context.Context,
		keyID uuid.UUID,
		labels []*model.KeyLabel,
	) error
	DeleteLabel(
		ctx context.Context,
		keyID uuid.UUID,
		labelName string,
	) (bool, error)
}

// LabelManager is an adapter that delegates to ResourceLabelManager
// Maintains backward compatibility while using the new unified resource_labels table
type LabelManager struct {
	repository     repo.Repo
	resourceLabels ResourceLabels
}

// NewLabelManager creates a new LabelManager that uses ResourceLabelManager
func NewLabelManager(
	repository repo.Repo,
	resourceLabels ResourceLabels,
) *LabelManager {
	return &LabelManager{
		repository:     repository,
		resourceLabels: resourceLabels,
	}
}

// DeleteLabel removes a label by key name for a specific key
func (m *LabelManager) DeleteLabel(
	ctx context.Context,
	keyID uuid.UUID,
	labelName string,
) (bool, error) {
	if labelName == "" {
		return false, ErrEmptyInputLabelDB
	}

	// Verify key exists
	key := &model.Key{ID: keyID}
	_, err := m.repository.First(ctx, key, *repo.NewQuery())
	if err != nil {
		return false, errs.Wrap(ErrGetKeyIDDB, err)
	}

	// Delete label using ResourceLabelManager
	return m.resourceLabels.DeleteLabel(ctx, model.ResourceTypeKey, keyID, labelName)
}

// CreateOrUpdateLabel creates or updates labels for a key
func (m *LabelManager) CreateOrUpdateLabel(
	ctx context.Context,
	keyID uuid.UUID,
	labels []*model.KeyLabel,
) error {
	// Verify key exists
	key := &model.Key{ID: keyID}
	ck := repo.NewCompositeKey().Where(repo.IDField, keyID)

	_, err := m.repository.First(ctx, key, *repo.NewQuery().
		Where(repo.NewCompositeKeyGroup(ck)))
	if err != nil {
		return errs.Wrap(ErrGettingKeyByID, err)
	}

	// Convert KeyLabel to ResourceLabel
	resourceLabels := make([]*model.ResourceLabel, 0, len(labels))
	for _, label := range labels {
		resourceLabels = append(resourceLabels, &model.ResourceLabel{
			ID:           label.ID,
			ResourceType: model.ResourceTypeKey,
			ResourceID:   keyID,
			Key:          label.Key,
			Value:        label.Value,
		})
	}

	// Delegate to ResourceLabelManager
	return m.resourceLabels.CreateOrUpdateLabels(ctx, model.ResourceTypeKey, keyID, resourceLabels)
}

// GetKeyLabels retrieves all labels for a key with pagination
func (m *LabelManager) GetKeyLabels(
	ctx context.Context,
	keyID uuid.UUID,
	pagination repo.Pagination,
) ([]*model.KeyLabel, int, error) {
	// Verify key exists
	key := &model.Key{ID: keyID}
	_, err := m.repository.First(ctx, key, *repo.NewQuery())
	if err != nil {
		return nil, 0, errs.Wrap(ErrGettingKeyByID, err)
	}

	// Get labels from ResourceLabelManager
	resourceLabels, count, err := m.resourceLabels.GetLabels(ctx, model.ResourceTypeKey, keyID, pagination)
	if err != nil {
		return nil, 0, err
	}

	// Convert ResourceLabel back to KeyLabel for backward compatibility
	keyLabels := make([]*model.KeyLabel, 0, len(resourceLabels))
	for _, rl := range resourceLabels {
		keyLabels = append(keyLabels, &model.KeyLabel{
			BaseLabel: model.BaseLabel{
				ID:         rl.ID,
				Key:        rl.Key,
				Value:      rl.Value,
				ResourceID: rl.ResourceID,
			},
			AutoTimeModel: model.AutoTimeModel{
				CreatedAt: rl.CreatedAt,
				UpdatedAt: rl.UpdatedAt,
			},
		})
	}

	return keyLabels, count, nil
}
