package manager

import (
	"context"
	"errors"

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
// It locks the row with DB lock, validates it, then deletes it in a Tx.
func (c *Pool) Pop(ctx context.Context) (*model.Keystore, error) {
	ks := &model.Keystore{}

	err := c.repo.Transaction(ctx, func(ctx context.Context) error {
		found, err := c.repo.First(ctx, ks, *repo.NewQuery().
			Order(repo.OrderField{Field: repo.CreatedField, Direction: repo.Desc}).
			WithLock(repo.LockForUpdateSkipLocked),
		)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				return ErrPoolIsDrained
			}
			return err
		}
		if !found {
			return ErrPoolIsDrained
		}

		if err := validateKeystore(ks); err != nil {
			return err
		}

		deleted, err := c.repo.Delete(ctx, ks, *repo.NewQuery())
		if err != nil {
			return err
		}
		if !deleted {
			return ErrPoolIsDrained
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return ks, nil
}

// validateKeystore checks whether a keystore entry is suitable for use.
func validateKeystore(_ *model.Keystore) error {
	// No checks needed for now.
	// Extend this function to add validation logic as the keystore schema evolves.
	return nil
}
