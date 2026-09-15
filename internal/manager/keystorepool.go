package manager

import (
	"context"
	"sync"
	"time"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
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
	c.mx.Lock()
	defer c.mx.Unlock()

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

// Pop `KeystoreConfiguration` from the pool and return it.
func (c *Pool) Pop(ctx context.Context) (*model.Keystore, error) {
	c.mx.Lock()
	defer c.mx.Unlock()

	ks := &model.Keystore{}

	// Only pop ACTIVE keystores from the pool
	compositeKey := repo.NewCompositeKey().Where(repo.StatusField, model.KeystoreStatusActive)
	query := repo.NewQuery().
		Where(repo.NewCompositeKeyGroup(compositeKey)).
		Order(repo.OrderField{
			Field:     repo.CreatedField,
			Direction: repo.Desc,
		})

	_, err := c.repo.First(ctx, ks, *query)
	if err != nil {
		return nil, errs.Wrap(ErrPoolIsDrained, err)
	}

	_, err = c.repo.Delete(ctx, ks, *repo.NewQuery())
	if err != nil {
		return nil, errs.Wrap(ErrCouldNotRemoveConfiguration, err)
	}

	return ks, nil
}

// CountPending returns the number of PENDING_ACTIVATION keystores.
func (c *Pool) CountPending(ctx context.Context) (int, error) {
	c.mx.Lock()
	defer c.mx.Unlock()

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
	c.mx.Lock()
	defer c.mx.Unlock()

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
	c.mx.Lock()
	defer c.mx.Unlock()

	_, err := c.repo.Patch(ctx, ks, *repo.NewQuery())
	if err != nil {
		return errs.Wrap(ErrCouldNotSaveConfiguration, err)
	}

	return nil
}

// MarkOrphanedKeystores marks keystores that have been PENDING for more than the specified duration as ORPHANED.
func (c *Pool) MarkOrphanedKeystores(ctx context.Context, maxAge time.Duration) (int, error) {
	c.mx.Lock()
	defer c.mx.Unlock()

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
	c.mx.Lock()
	defer c.mx.Unlock()

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

