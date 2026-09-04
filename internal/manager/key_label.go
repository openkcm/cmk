package manager

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

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

type LabelManager struct {
	repository repo.Repo
}

func NewLabelManager(
	repository repo.Repo,
) *LabelManager {
	return &LabelManager{
		repository: repository,
	}
}

func (m *LabelManager) DeleteLabel(
	ctx context.Context,
	keyID uuid.UUID,
	labelName string,
) (bool, error) {
	if labelName == "" {
		return false, ErrEmptyInputLabelDB
	}

	key := &model.Key{ID: keyID}

	_, err := m.repository.First(ctx, key, *repo.NewQuery())
	if err != nil {
		return false, errs.Wrap(ErrGetKeyIDDB, err)
	}

	label := &model.KeyLabel{}

	ck := repo.NewCompositeKey().
		Where(repo.KeyField, labelName).
		Where(repo.ResourceIDField, keyID)

	// Primary delete: from labels table
	ok, err := m.repository.Delete(
		ctx,
		label,
		*repo.NewQuery().
			Where(repo.NewCompositeKeyGroup(ck)),
	)
	if err != nil {
		return false, errs.Wrap(ErrDeleteLabelDB, err)
	}

	// Double-write: delete from resource_labels (best-effort)
	m.syncDeleteResourceLabel(ctx, keyID, labelName)

	return ok, nil
}

func (m *LabelManager) CreateOrUpdateLabel(
	ctx context.Context,
	keyID uuid.UUID,
	labels []*model.KeyLabel,
) error {
	key := &model.Key{ID: keyID}
	ck := repo.NewCompositeKey().Where(repo.IDField, keyID)

	_, err := m.repository.First(ctx, key, *repo.NewQuery().
		Where(repo.NewCompositeKeyGroup(ck)))
	if err != nil {
		return errs.Wrap(ErrGettingKeyByID, err)
	}

	err = m.repository.Transaction(ctx, func(ctx context.Context) error {
		for _, label := range labels {
			l := &model.KeyLabel{}
			ck = repo.NewCompositeKey().Where(repo.KeyField, label.Key).Where(repo.ResourceIDField, keyID)

			_, err := m.repository.First(
				ctx,
				l,
				*repo.NewQuery().
					Where(repo.NewCompositeKeyGroup(ck)),
			)
			if err != nil {
				if !errors.Is(err, repo.ErrNotFound) {
					return errs.Wrap(ErrFetchLabel, err)
				}

				// Primary write: create in labels table
				err := m.repository.Create(ctx, label)
				if err != nil {
					return errs.Wrap(ErrInsertLabel, err)
				}

				// Double-write: sync to resource_labels (best-effort)
				m.syncCreateResourceLabel(ctx, keyID, label)
			} else {
				l.Value = label.Value

				// Primary write: update in labels table
				_, err := m.repository.Patch(
					ctx,
					l,
					*repo.NewQuery().UpdateAll(true),
				)
				if err != nil {
					return errs.Wrap(ErrUpdateLabelDB, err)
				}

				// Double-write: sync to resource_labels (best-effort)
				m.syncUpdateResourceLabel(ctx, keyID, label)
			}
		}

		return nil
	})
	if err != nil {
		return errs.Wrap(ErrUpdateLabelDB, err)
	}

	return nil
}

func (m *LabelManager) GetKeyLabels(
	ctx context.Context,
	keyID uuid.UUID,
	pagination repo.Pagination,
) ([]*model.KeyLabel, int, error) {
	key := &model.Key{ID: keyID}

	_, err := m.repository.First(ctx, key, *repo.NewQuery())
	if err != nil {
		return nil, 0, errs.Wrap(ErrGettingKeyByID, err)
	}

	ck := repo.NewCompositeKey().
		Where(repo.ResourceIDField, keyID)

	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))

	return repo.ListAndCount(ctx, m.repository, pagination, model.KeyLabel{}, query)
}

// syncDeleteResourceLabel removes a label from the resource_labels table
func (m *LabelManager) syncDeleteResourceLabel(ctx context.Context, keyID uuid.UUID, labelName string) {
	ck := repo.NewCompositeKey().
		Where(repo.ResourceTypeField, model.ResourceTypeKeyConfig).
		Where(repo.ResourceIDField, keyID).
		Where(repo.KeyField, labelName)

	_, _ = m.repository.Delete(ctx, &model.ResourceLabel{}, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
}

// syncCreateResourceLabel writes a new label to the resource_labels table
func (m *LabelManager) syncCreateResourceLabel(ctx context.Context, keyID uuid.UUID, label *model.KeyLabel) {
	// Avoid overwriting tag rows if a KeyLabel ever uses the reserved key
	if label.Key == model.SystemTagKey {
		return
	}
	resourceLabel := &model.ResourceLabel{
		ID:           uuid.New(),
		ResourceType: model.ResourceTypeKeyConfig,
		ResourceID:   keyID,
		Key:          label.Key,
		Value:        label.Value,
	}
	_ = m.repository.Create(ctx, resourceLabel)
}

// syncUpdateResourceLabel updates a label in the resource_labels table
func (m *LabelManager) syncUpdateResourceLabel(ctx context.Context, keyID uuid.UUID, label *model.KeyLabel) {
	// Avoid overwriting tag rows if a KeyLabel ever uses the reserved key
	if label.Key == model.SystemTagKey {
		return
	}
	// Find existing resource label
	rl := &model.ResourceLabel{}
	ck := repo.NewCompositeKey().
		Where(repo.ResourceTypeField, model.ResourceTypeKeyConfig).
		Where(repo.ResourceIDField, keyID).
		Where(repo.KeyField, label.Key)

	_, err := m.repository.First(ctx, rl, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			// Doesn't exist, create it
			m.syncCreateResourceLabel(ctx, keyID, label)
		}
		return
	}

	// Update value
	rl.Value = label.Value
	_, _ = m.repository.Patch(ctx, rl, *repo.NewQuery().UpdateAll(true))
}
