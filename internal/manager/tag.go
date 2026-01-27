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
	_, err := m.r.Delete(ctx, &model.Tag{ID: itemID}, *repo.NewQuery())
	if err != nil {
		return errs.Wrap(ErrDeletingTags, err)
	}

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

	return m.r.Set(ctx, &model.Tag{ID: itemID, Values: bytes})
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
