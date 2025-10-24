package manager

import (
	"context"
	"sync"

	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
)

// Pool stores available configurations.
type Pool struct {
	repo repo.Repo
	mx   sync.Mutex
}

// NewPool creates a new instance of Pool.
func NewPool(repo repo.Repo) *Pool {
	return &Pool{
		repo: repo,
		mx:   sync.Mutex{},
	}
}

func (c *Pool) Count(ctx context.Context) (int, error) {
	c.mx.Lock()
	defer c.mx.Unlock()

	config := &model.KeystoreConfiguration{}

	count, err := c.repo.List(ctx, &model.KeystoreConfiguration{}, config, *repo.NewQuery().SetLimit(0))
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Add `KeystoreConfiguration` to the pool.
func (c *Pool) Add(ctx context.Context, cfg *model.KeystoreConfiguration) (*model.KeystoreConfiguration, error) {
	c.mx.Lock()
	defer c.mx.Unlock()

	err := c.repo.Create(ctx, cfg)
	if err != nil {
		return nil, errs.Wrap(ErrCouldNotSaveConfiguration, err)
	}

	return cfg, nil
}

// Pop `KeystoreConfiguration` from the pool and return it.
func (c *Pool) Pop(ctx context.Context) (*model.KeystoreConfiguration, error) {
	c.mx.Lock()
	defer c.mx.Unlock()

	cfg := &model.KeystoreConfiguration{}

	_, err := c.repo.First(ctx, cfg, *repo.NewQuery().Order(repo.OrderField{
		Field:     repo.CreatedField,
		Direction: repo.Desc,
	}))
	if err != nil {
		return nil, errs.Wrap(ErrPoolIsDrained, err)
	}

	_, err = c.repo.Delete(ctx, cfg, *repo.NewQuery())
	if err != nil {
		return nil, errs.Wrap(ErrCouldNotRemoveConfiguration, err)
	}

	return cfg, nil
}
