package manager_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	repoMock "github.com/openkcm/cmk/internal/repo/mock"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

var itemID = uuid.New()

func TestPool_Add(t *testing.T) {
	t.Run("should save Configuration in repo", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)
		item := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = itemID
		})

		addedItem, err := testPool.Add(t.Context(), item)

		assert.NoError(t, err)
		assert.Equal(t, item, addedItem)
	})
}

func TestPool_Pop(t *testing.T) {
	t.Run("should get first available Configuration from repo", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)

		item := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = itemID
		})
		addedItem, err := testPool.Add(t.Context(), item)
		assert.NoError(t, err)
		assert.Equal(t, item, addedItem)

		wg := sync.WaitGroup{}
		wg.Add(2)

		var foundConfig *model.Keystore

		var encounteredError bool

		go func() {
			receivedItem, popErr := testPool.Pop(t.Context())
			if popErr != nil {
				encounteredError = true
			} else {
				foundConfig = receivedItem
			}

			wg.Done()
		}()

		go func() {
			receivedItem, popErr := testPool.Pop(t.Context())
			if popErr != nil {
				encounteredError = true
			} else if receivedItem != nil {
				foundConfig = receivedItem
			}

			wg.Done()
		}()

		wg.Wait()

		assert.Equal(t, addedItem.ID, foundConfig.ID)
		assert.Equal(t, addedItem.Provider, foundConfig.Provider)

		var expectedValue, actualValue map[string]any

		err = json.Unmarshal(addedItem.Config, &expectedValue)
		assert.NoError(t, err)
		err = json.Unmarshal(foundConfig.Config, &actualValue)
		assert.NoError(t, err)
		assert.Equal(t, expectedValue, actualValue)
		assert.True(t, encounteredError)
	})

	t.Run("ErrPoolIsDrained on empty pool", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)

		_, err := testPool.Pop(t.Context())

		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrPoolIsDrained)
	})

	t.Run("returns First error as-is on unexpected repo failure", func(t *testing.T) {
		inner := repoMock.NewInMemoryRepository()
		testPool := manager.NewPool(&firstErrorRepo{inner})

		_, err := testPool.Pop(t.Context())

		assert.Error(t, err)
		assert.NotErrorIs(t, err, manager.ErrPoolIsDrained)
	})

	t.Run("returns error on Delete failure", func(t *testing.T) {
		inner := repoMock.NewInMemoryRepository()
		testPool := manager.NewPool(&deleteErrorRepo{inner})

		_, err := testPool.Pop(t.Context())

		assert.Error(t, err)
		assert.NotErrorIs(t, err, manager.ErrPoolIsDrained)
	})

	t.Run("ErrPoolIsDrained when Delete reports no row deleted", func(t *testing.T) {
		inner := repoMock.NewInMemoryRepository()
		testPool := manager.NewPool(&deleteNotFoundRepo{inner})

		_, err := testPool.Pop(t.Context())

		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrPoolIsDrained)
	})

	t.Run("ErrPoolIsDrained race condition", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)

		item := testutils.NewKeystore(func(kc *model.Keystore) {})
		_, err := testPool.Add(t.Context(), item)
		assert.NoError(t, err)

		// Simulate concurrent depletion: pop the only item before the second pop runs.
		_, err = testPool.Pop(t.Context())
		assert.NoError(t, err)

		_, err = testPool.Pop(t.Context())

		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrPoolIsDrained)
	})
}

// Sentinel errors for test mocks.
var (
	errSimulatedFirst  = errors.New("simulated first error")
	errSimulatedDelete = errors.New("simulated delete error")
)

// Mocks for testing error scenarios in the repo layer.

// firstErrorRepo overrides First to return a non-ErrNotFound error.
type firstErrorRepo struct {
	*repoMock.InMemoryRepository
}

func (r *firstErrorRepo) First(_ context.Context, _ repo.Resource, _ repo.Query) (bool, error) {
	return false, errSimulatedFirst
}

func (r *firstErrorRepo) Transaction(ctx context.Context, txFunc repo.TransactionFunc) error {
	return txFunc(ctx)
}

// deleteErrorRepo overrides Delete to return an error after First succeeds.
type deleteErrorRepo struct {
	*repoMock.InMemoryRepository
}

func (r *deleteErrorRepo) First(_ context.Context, resource repo.Resource, _ repo.Query) (bool, error) {
	if ks, ok := resource.(*model.Keystore); ok {
		ks.ID = uuid.New()
	}
	return true, nil
}

func (r *deleteErrorRepo) Delete(_ context.Context, _ repo.Resource, _ repo.Query) (bool, error) {
	return false, errSimulatedDelete
}

func (r *deleteErrorRepo) Transaction(ctx context.Context, txFunc repo.TransactionFunc) error {
	return txFunc(ctx)
}

// deleteNotFoundRepo overrides Delete to report no row was deleted (e.g. concurrent deletion).
type deleteNotFoundRepo struct {
	*repoMock.InMemoryRepository
}

func (r *deleteNotFoundRepo) First(_ context.Context, resource repo.Resource, _ repo.Query) (bool, error) {
	if ks, ok := resource.(*model.Keystore); ok {
		ks.ID = uuid.New()
	}
	return true, nil
}

func (r *deleteNotFoundRepo) Delete(_ context.Context, _ repo.Resource, _ repo.Query) (bool, error) {
	return false, nil
}

func (r *deleteNotFoundRepo) Transaction(ctx context.Context, txFunc repo.TransactionFunc) error {
	return txFunc(ctx)
}
