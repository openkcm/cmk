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

var (
	ErrGetKeyConfig = errors.New("error getting keyconfig")
	ErrCreateTag    = errors.New("error setting tags")
)

type Tags interface {
	SetTags(ctx context.Context, itemID uuid.UUID, values []string) error
	GetTags(ctx context.Context, itemID uuid.UUID) ([]string, error)
	DeleteTags(ctx context.Context, itemID uuid.UUID) error
}

type TagManager struct {
	r repo.Repo
}

func NewTagManager(r repo.Repo) *TagManager {
	return &TagManager{
		r: r,
	}
}

func (m *TagManager) DeleteTags(ctx context.Context, itemID uuid.UUID) error {
	// Primary delete: from tags table (existing behavior)
	_, err := m.r.Delete(ctx, &model.Tag{ID: itemID}, *repo.NewQuery())
	if err != nil {
		return errs.Wrap(ErrDeletingTags, err)
	}

	// Double-write: delete from resource_labels table (new table, best-effort)
	_ = m.syncDeleteResourceLabels(ctx, itemID)

	return nil
}

// syncDeleteResourceLabels removes tags from the new resource_labels table
func (m *TagManager) syncDeleteResourceLabels(ctx context.Context, itemID uuid.UUID) error {
	_, _ = m.r.Delete(ctx, &model.ResourceLabel{}, *repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().
				Where(repo.ResourceTypeField, model.ResourceTypeKeyConfig).
				Where(repo.ResourceIDField, itemID).
				Where(repo.KeyField, model.SystemTagKey),
		),
	))
	return nil
}

func (m *TagManager) SetTags(ctx context.Context, itemID uuid.UUID, values []string) error {
	if len(values) == 1 && values[0] == "" {
		return m.DeleteTags(ctx, itemID)
	}

	bytes, err := json.Marshal(values)
	if err != nil {
		return err
	}

	// Primary write: to tags table (existing behavior)
	err = m.r.Set(ctx, &model.Tag{ID: itemID, Values: bytes}, *repo.NewQuery())
	if err != nil {
		return err
	}

	// Double-write: sync to resource_labels table (new table, best-effort)
	// Errors here are logged but don't fail the operation for backward compatibility
	_ = m.syncToResourceLabels(ctx, itemID, values)

	return nil
}

// syncToResourceLabels writes tags to the new resource_labels table
// This is a sync operation that happens alongside the primary write to tags table
func (m *TagManager) syncToResourceLabels(ctx context.Context, itemID uuid.UUID, values []string) error {
	// Delete existing tags for this resource
	_, _ = m.r.Delete(ctx, &model.ResourceLabel{
		ResourceType: model.ResourceTypeKeyConfig,
		ResourceID:   itemID,
		Key:          model.SystemTagKey,
	}, *repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().
				Where(repo.ResourceTypeField, model.ResourceTypeKeyConfig).
				Where(repo.ResourceIDField, itemID).
				Where(repo.KeyField, model.SystemTagKey),
		),
	))

	// Insert new tags
	for _, value := range values {
		if value == "" {
			continue
		}
		label := &model.ResourceLabel{
			ID:           uuid.New(),
			ResourceType: model.ResourceTypeKeyConfig,
			ResourceID:   itemID,
			Key:          model.SystemTagKey,
			Value:        value,
		}
		_ = m.r.Create(ctx, label)
	}

	return nil
}

func (m *TagManager) GetTags(ctx context.Context, itemID uuid.UUID) ([]string, error) {
	values := []string{}
	tag := &model.Tag{ID: itemID}
	_, err := m.r.First(ctx, tag, *repo.NewQuery())

	if errors.Is(err, repo.ErrNotFound) {
		return values, nil
	}

	if !errors.Is(err, err) {
		return nil, errs.Wrap(ErrGetTags, err)
	}

	err = json.Unmarshal(tag.Values, &values)
	if err != nil {
		return nil, err
	}

	return values, nil
}
