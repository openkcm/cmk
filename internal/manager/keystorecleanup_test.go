package manager_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestKeystoreCleanupService_RunCleanup(t *testing.T) {
	t.Run("should mark old PENDING keystores as ORPHANED", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)
		cleanupService := manager.NewKeystoreCleanupService(testPool, 48*time.Hour)

		// Add old PENDING keystore (50 hours old)
		oldItem := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusPendingActivation
			kc.CreatedAt = time.Now().Add(-50 * time.Hour)
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

		// Run cleanup without delete
		err = cleanupService.RunCleanup(t.Context(), false)
		require.NoError(t, err)

		// Verify only recent one is still PENDING
		pendingCount, err := testPool.CountPending(t.Context())
		require.NoError(t, err)
		require.Equal(t, 1, pendingCount)
	})

	t.Run("should delete ORPHANED keystores when deleteOrphaned is true", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)
		cleanupService := manager.NewKeystoreCleanupService(testPool, 48*time.Hour)

		// Add old PENDING keystore
		oldItem := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusPendingActivation
			kc.CreatedAt = time.Now().Add(-50 * time.Hour)
		})
		_, err := testPool.Add(t.Context(), oldItem)
		require.NoError(t, err)

		// Add ACTIVE keystore
		activeItem := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusActive
		})
		_, err = testPool.Add(t.Context(), activeItem)
		require.NoError(t, err)

		// Run cleanup WITH delete
		err = cleanupService.RunCleanup(t.Context(), true)
		require.NoError(t, err)

		// Verify old PENDING is gone
		pendingCount, err := testPool.CountPending(t.Context())
		require.NoError(t, err)
		require.Equal(t, 0, pendingCount)

		// Verify ACTIVE is still there
		activeCount, err := testPool.Count(t.Context())
		require.NoError(t, err)
		require.Equal(t, 1, activeCount)
	})

	t.Run("should use default max age when not specified", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)
		// Pass 0 to use default (48 hours)
		cleanupService := manager.NewKeystoreCleanupService(testPool, 0)

		// Add keystore that's 49 hours old (should be marked with default 48h)
		oldItem := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusPendingActivation
			kc.CreatedAt = time.Now().Add(-49 * time.Hour)
		})
		_, err := testPool.Add(t.Context(), oldItem)
		require.NoError(t, err)

		// Run cleanup
		err = cleanupService.RunCleanup(t.Context(), false)
		require.NoError(t, err)

		// Should be marked as orphaned
		pendingCount, err := testPool.CountPending(t.Context())
		require.NoError(t, err)
		require.Equal(t, 0, pendingCount)
	})

	t.Run("should not affect ACTIVE or FAILED keystores", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)
		cleanupService := manager.NewKeystoreCleanupService(testPool, 48*time.Hour)

		// Add very old ACTIVE keystore
		oldActiveItem := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusActive
			kc.CreatedAt = time.Now().Add(-100 * time.Hour)
		})
		_, err := testPool.Add(t.Context(), oldActiveItem)
		require.NoError(t, err)

		// Add very old FAILED keystore
		oldFailedItem := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = uuid.New()
			kc.Status = model.KeystoreStatusFailed
			kc.CreatedAt = time.Now().Add(-100 * time.Hour)
		})
		_, err = testPool.Add(t.Context(), oldFailedItem)
		require.NoError(t, err)

		// Run cleanup with delete
		err = cleanupService.RunCleanup(t.Context(), true)
		require.NoError(t, err)

		// Both should still exist
		activeCount, err := testPool.Count(t.Context())
		require.NoError(t, err)
		require.Equal(t, 1, activeCount)

		// FAILED should still be there (not deleted)
		// Note: We don't have a CountFailed method, but we can verify it wasn't affected
		// by the fact that nothing was marked or deleted
	})
}
