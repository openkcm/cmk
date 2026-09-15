package manager

import (
	"context"
	"errors"
	"time"

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
	// Only count ACTIVE keystores in the pool
	compositeKey := repo.NewCompositeKey().Where(repo.StatusField, model.KeystoreStatusActive)
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(compositeKey))

	count, err := c.repo.Count(ctx, &model.Keystore{}, *query)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Add `KeystoreConfiguration` to the pool.
func (c *Pool) Add(ctx context.Context, ks *model.Keystore) (*model.Keystore, error) {
	// Set default status to ACTIVE if not specified
	if ks.Status == "" {
		ks.Status = model.KeystoreStatusActive
	}

	err := c.repo.Create(ctx, ks)
	if err != nil {
		return nil, errs.Wrap(ErrCouldNotSaveConfiguration, err)
	}

	return ks, nil
}

// Pop removes one ACTIVE `KeystoreConfiguration` from the pool and returns it.
// It locks the row with DB lock, validates it, then deletes it in a Tx.
func (c *Pool) Pop(ctx context.Context) (*model.Keystore, error) {
	ks := &model.Keystore{}

	err := c.repo.Transaction(ctx, func(ctx context.Context) error {
		// Only pop ACTIVE keystores from the pool
		compositeKey := repo.NewCompositeKey().Where(repo.StatusField, model.KeystoreStatusActive)
		found, err := c.repo.First(ctx, ks, *repo.NewQuery().
			Where(repo.NewCompositeKeyGroup(compositeKey)).
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

// CountPending returns the number of PENDING_ACTIVATION keystores.
func (c *Pool) CountPending(ctx context.Context) (int, error) {
	compositeKey := repo.NewCompositeKey().Where(repo.StatusField, model.KeystoreStatusPendingActivation)
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(compositeKey))

	count, err := c.repo.Count(ctx, &model.Keystore{}, *query)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetPending returns all PENDING_ACTIVATION keystores.
func (c *Pool) GetPending(ctx context.Context) ([]*model.Keystore, error) {
	var keystores []*model.Keystore

	compositeKey := repo.NewCompositeKey().Where(repo.StatusField, model.KeystoreStatusPendingActivation)
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(compositeKey))

	err := c.repo.List(ctx, &model.Keystore{}, &keystores, *query)
	if err != nil {
		return nil, err
	}

	return keystores, nil
}

// Update updates a keystore in the pool.
func (c *Pool) Update(ctx context.Context, ks *model.Keystore) error {
	_, err := c.repo.Patch(ctx, ks, *repo.NewQuery())
	if err != nil {
		return errs.Wrap(ErrCouldNotSaveConfiguration, err)
	}

	return nil
}

// MarkOrphanedKeystores marks keystores that have been PENDING for more than the specified duration as ORPHANED.
func (c *Pool) MarkOrphanedKeystores(ctx context.Context, maxAge time.Duration) (int, error) {
	var keystores []*model.Keystore

	// Find PENDING keystores older than maxAge
	cutoffTime := time.Now().Add(-maxAge)
	compositeKey := repo.NewCompositeKey().
		Where(repo.StatusField, model.KeystoreStatusPendingActivation).
		Where(repo.CreatedField, cutoffTime, repo.Lt)
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(compositeKey))

	err := c.repo.List(ctx, &model.Keystore{}, &keystores, *query)
	if err != nil {
		return 0, err
	}

	// Mark each as ORPHANED
	for _, ks := range keystores {
		ks.Status = model.KeystoreStatusOrphaned
		_, err = c.repo.Patch(ctx, ks, *repo.NewQuery())
		if err != nil {
			return 0, err
		}
	}

	return len(keystores), nil
}

// DeleteOrphanedKeystores permanently deletes keystores marked as ORPHANED.
func (c *Pool) DeleteOrphanedKeystores(ctx context.Context) (int, error) {
	var keystores []*model.Keystore

	compositeKey := repo.NewCompositeKey().Where(repo.StatusField, model.KeystoreStatusOrphaned)
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(compositeKey))

	err := c.repo.List(ctx, &model.Keystore{}, &keystores, *query)
	if err != nil {
		return 0, err
	}

	// Delete each orphaned keystore
	for _, ks := range keystores {
		_, err = c.repo.Delete(ctx, ks, *repo.NewQuery())
		if err != nil {
			return 0, err
		}
	}

	return len(keystores), nil
}
