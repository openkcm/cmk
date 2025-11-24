package manager

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/utils/sanitise"
)

var (
	ErrGetKeyConfig = errors.New("error getting keyconfig")
	ErrCreateTag    = errors.New("error creating tags")
)

type Parent interface {
	IsSharedModel() bool
	TableName() string
	SetID(id uuid.UUID)
}

// createTagsByKeyConfiguration creates tags. If tags already exist, they are replaced.
func createTags[Tparent Parent, Ttag any](
	ctx context.Context,
	mrepo repo.Repo,
	p Tparent,
	id uuid.UUID,
	tags []*Ttag,
) error {
	p.SetID(id)

	_, err := mrepo.Patch(
		ctx,
		p,
		*repo.NewQuery().Associate(repo.Association{
			Field: "Tags",
			Value: tags,
		}),
	)
	if err != nil {
		return errs.Wrap(ErrCreateTag, err)
	}

	return nil
}

func getTags[Tparent Parent](ctx context.Context, mrepo repo.Repo,
	id uuid.UUID, p Tparent,
) error {
	ck := repo.NewCompositeKey().
		Where(repo.IDField, id)

	_, err := mrepo.First(ctx,
		p,
		*repo.NewQuery().
			Preload(repo.Preload{"Tags"}).
			Where(repo.NewCompositeKeyGroup(ck)),
	)
	if err != nil {
		return errs.Wrap(ErrGetKeyConfig, err)
	}

	// We don't assign to the return since updated in situ
	// See limitation in sanitise.go around maps.
	_, err = sanitise.Stringlikes(p)
	if err != nil {
		return errs.Wrap(ErrCreateTag, err)
	}

	return nil
}
