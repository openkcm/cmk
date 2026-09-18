package manager

import (
	"context"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

// Pool stores available configurations.
type Pool struct {
	repo repo.Repo
}

// NewPool creates a new instance of Pool.
func NewPool(repo repo.Repo) *Pool {
	return &Pool{
		repo: repo,
	}
}

func (c *Pool) Count(ctx context.Context) (int, error) {
	count, err := c.repo.Count(ctx, &model.Keystore{}, *repo.NewQuery())
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Add `KeystoreConfiguration` to the pool.
func (c *Pool) Add(ctx context.Context, ks *model.Keystore) (*model.Keystore, error) {
	err := c.repo.Create(ctx, ks)
	if err != nil {
		return nil, errs.Wrap(ErrCouldNotSaveConfiguration, err)
	}

	return ks, nil
}

// Pop removes one `KeystoreConfiguration` from the pool and returns it.
// The operation is atomic at the database level
func (c *Pool) Pop(ctx context.Context) (*model.Keystore, error) {
	ks, err := c.repo.PopKeystore(ctx)
	if err != nil {
		return nil, errs.Wrap(ErrPoolIsDrained, err)
	}

	return ks, nil
}
