package manager

import (
	"context"
	"log/slog"
	"time"

	"github.com/openkcm/cmk/internal/log"
)

const (
	// DefaultOrphanedKeystoreMaxAge is the default duration after which a PENDING keystore is considered orphaned
	DefaultOrphanedKeystoreMaxAge = 48 * time.Hour
)

// KeystoreCleanupService handles cleanup of orphaned keystores
type KeystoreCleanupService struct {
	pool   *Pool
	maxAge time.Duration
}

// NewKeystoreCleanupService creates a new cleanup service
func NewKeystoreCleanupService(pool *Pool, maxAge time.Duration) *KeystoreCleanupService {
	if maxAge == 0 {
		maxAge = DefaultOrphanedKeystoreMaxAge
	}
	return &KeystoreCleanupService{
		pool:   pool,
		maxAge: maxAge,
	}
}

// RunCleanup marks old PENDING keystores as ORPHANED and optionally deletes them
func (s *KeystoreCleanupService) RunCleanup(ctx context.Context, deleteOrphaned bool) error {
	log.Info(ctx, "Starting keystore cleanup",
		slog.Duration("maxAge", s.maxAge),
		slog.Bool("deleteOrphaned", deleteOrphaned),
	)

	// Mark old PENDING keystores as ORPHANED
	markedCount, err := s.pool.MarkOrphanedKeystores(ctx, s.maxAge)
	if err != nil {
		log.Error(ctx, "Failed to mark orphaned keystores", err)
		return err
	}

	log.Info(ctx, "Marked keystores as orphaned",
		slog.Int("count", markedCount),
	)

	// Optionally delete ORPHANED keystores
	if deleteOrphaned {
		deletedCount, err := s.pool.DeleteOrphanedKeystores(ctx)
		if err != nil {
			log.Error(ctx, "Failed to delete orphaned keystores", err)
			return err
		}

		log.Info(ctx, "Deleted orphaned keystores",
			slog.Int("count", deletedCount),
		)
	}

	return nil
}
