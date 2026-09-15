package manager_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestPool_Add_WithStatus(t *testing.T) {
	t.Run("should default status to ACTIVE when not specified", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)
		item := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			// Status not set
		})

		addedItem, err := testPool.Add(t.Context(), item)

		require.NoError(t, err)
		require.Equal(t, model.KeystoreStatusActive, addedItem.Status)
	})

	t.Run("should preserve explicitly set status", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)
		item := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusPendingActivation
		})

		addedItem, err := testPool.Add(t.Context(), item)

		require.NoError(t, err)
		require.Equal(t, model.KeystoreStatusPendingActivation, addedItem.Status)
	})
}

func TestPool_Count_OnlyCountsActive(t *testing.T) {
	t.Run("should only count ACTIVE keystores", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)

		// Add ACTIVE keystore
		activeItem := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusActive
		})
		_, err := testPool.Add(t.Context(), activeItem)
		require.NoError(t, err)

		// Add PENDING keystore
		pendingItem := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusPendingActivation
		})
		_, err = testPool.Add(t.Context(), pendingItem)
		require.NoError(t, err)

		// Add ORPHANED keystore
		orphanedItem := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusOrphaned
		})
		_, err = testPool.Add(t.Context(), orphanedItem)
		require.NoError(t, err)

		// Count should only return 1 (ACTIVE)
		count, err := testPool.Count(t.Context())
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})
}

func TestPool_Pop_OnlyPopsActive(t *testing.T) {
	t.Run("should only pop ACTIVE keystores", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)

		// Add PENDING keystore (should not be popped)
		pendingItem := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusPendingActivation
		})
		_, err := testPool.Add(t.Context(), pendingItem)
		require.NoError(t, err)

		// Add ACTIVE keystore (should be popped)
		activeItem := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusActive
		})
		_, err = testPool.Add(t.Context(), activeItem)
		require.NoError(t, err)

		// Pop should return the ACTIVE one
		poppedItem, err := testPool.Pop(t.Context())
		require.NoError(t, err)
		require.Equal(t, activeItem.ID, poppedItem.ID)
		require.Equal(t, model.KeystoreStatusActive, poppedItem.Status)

		// Second pop should fail (pending is still there but not popped)
		_, err = testPool.Pop(t.Context())
		require.Error(t, err)
	})
}

func TestPool_CountPending(t *testing.T) {
	t.Run("should count only PENDING_ACTIVATION keystores", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)

		// Add ACTIVE keystore
		activeItem := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusActive
		})
		_, err := testPool.Add(t.Context(), activeItem)
		require.NoError(t, err)

		// Add PENDING keystores
		for i := 0; i < 3; i++ {
			pendingItem := testutils.NewKeystore(func(kc *model.Keystore) {
				kc.ID = uuid.New()
				kc.Status = model.KeystoreStatusPendingActivation
			})
			_, err = testPool.Add(t.Context(), pendingItem)
			require.NoError(t, err)
		}

		// CountPending should return 3
		count, err := testPool.CountPending(t.Context())
		require.NoError(t, err)
		require.Equal(t, 3, count)
	})
}

func TestPool_GetPending(t *testing.T) {
	t.Run("should return all PENDING_ACTIVATION keystores", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)

		// Add ACTIVE keystore
		activeItem := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusActive
		})
		_, err := testPool.Add(t.Context(), activeItem)
		require.NoError(t, err)

		// Add PENDING keystores
		pendingIDs := []uuid.UUID{}
		for i := 0; i < 2; i++ {
			id := uuid.New()
			pendingIDs = append(pendingIDs, id)
			pendingItem := testutils.NewKeystore(func(kc *model.Keystore) {
				kc.ID = id
				kc.Status = model.KeystoreStatusPendingActivation
			})
			_, err = testPool.Add(t.Context(), pendingItem)
			require.NoError(t, err)
		}

		// GetPending should return 2 items
		pending, err := testPool.GetPending(t.Context())
		require.NoError(t, err)
		require.Len(t, pending, 2)

		// Verify all returned items are PENDING
		for _, ks := range pending {
			require.Equal(t, model.KeystoreStatusPendingActivation, ks.Status)
		}
	})
}

func TestPool_Update(t *testing.T) {
	t.Run("should update keystore status", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)

		// Add PENDING keystore
		item := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusPendingActivation
		})
		addedItem, err := testPool.Add(t.Context(), item)
		require.NoError(t, err)

		// Update to ACTIVE
		addedItem.Status = model.KeystoreStatusActive
		err = testPool.Update(t.Context(), addedItem)
		require.NoError(t, err)

		// Verify it's now ACTIVE and can be popped
		poppedItem, err := testPool.Pop(t.Context())
		require.NoError(t, err)
		require.Equal(t, addedItem.ID, poppedItem.ID)
		require.Equal(t, model.KeystoreStatusActive, poppedItem.Status)
	})
}

func TestPool_MarkOrphanedKeystores(t *testing.T) {
	t.Run("should mark old PENDING keystores as ORPHANED", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)

		// Add old PENDING keystore (49 hours old)
		oldItem := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusPendingActivation
			kc.CreatedAt = time.Now().Add(-49 * time.Hour)
		})
		_, err := testPool.Add(t.Context(), oldItem)
		require.NoError(t, err)

		// Add recent PENDING keystore (1 hour old)
		recentItem := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusPendingActivation
			kc.CreatedAt = time.Now().Add(-1 * time.Hour)
		})
		_, err = testPool.Add(t.Context(), recentItem)
		require.NoError(t, err)

		// Mark orphaned (max age 48 hours)
		count, err := testPool.MarkOrphanedKeystores(t.Context(), 48*time.Hour)
		require.NoError(t, err)
		require.Equal(t, 1, count) // Only 1 marked

		// Verify pending count is now 1 (recent one)
		pendingCount, err := testPool.CountPending(t.Context())
		require.NoError(t, err)
		require.Equal(t, 1, pendingCount)
	})

	t.Run("should not mark ACTIVE keystores as ORPHANED", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)

		// Add old ACTIVE keystore
		oldActiveItem := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusActive
			kc.CreatedAt = time.Now().Add(-100 * time.Hour)
		})
		_, err := testPool.Add(t.Context(), oldActiveItem)
		require.NoError(t, err)

		// Mark orphaned
		count, err := testPool.MarkOrphanedKeystores(t.Context(), 48*time.Hour)
		require.NoError(t, err)
		require.Equal(t, 0, count) // Nothing marked

		// Verify ACTIVE count is still 1
		activeCount, err := testPool.Count(t.Context())
		require.NoError(t, err)
		require.Equal(t, 1, activeCount)
	})
}

func TestPool_DeleteOrphanedKeystores(t *testing.T) {
	t.Run("should delete ORPHANED keystores", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)

		// Add ORPHANED keystores
		for i := 0; i < 3; i++ {
			orphanedItem := testutils.NewKeystore(func(kc *model.Keystore) {
				kc.ID = uuid.New()
				kc.Status = model.KeystoreStatusOrphaned
			})
			_, err := testPool.Add(t.Context(), orphanedItem)
			require.NoError(t, err)
		}

		// Add ACTIVE keystore (should not be deleted)
		activeItem := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusActive
		})
		_, err := testPool.Add(t.Context(), activeItem)
		require.NoError(t, err)

		// Delete orphaned
		count, err := testPool.DeleteOrphanedKeystores(t.Context())
		require.NoError(t, err)
		require.Equal(t, 3, count)

		// Verify ACTIVE is still there
		activeCount, err := testPool.Count(t.Context())
		require.NoError(t, err)
		require.Equal(t, 1, activeCount)
	})
}
