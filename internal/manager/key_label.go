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

	ok, err := m.repository.Delete(
		ctx,
		label,
		*repo.NewQuery().
			Where(repo.NewCompositeKeyGroup(ck)),
	)
	if err != nil {
		return false, errs.Wrap(ErrDeleteLabelDB, err)
	}

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

				err := m.repository.Create(ctx, label)
				if err != nil {
					return errs.Wrap(ErrInsertLabel, err)
				}
			} else {
				l.Value = label.Value

				_, err := m.repository.Patch(
					ctx,
					l,
					*repo.NewQuery().UpdateAll(true),
				)
				if err != nil {
					return errs.Wrap(ErrUpdateLabelDB, err)
				}
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
