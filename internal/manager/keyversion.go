package manager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
	"github.com/openkcm/cmk/internal/repo"
)

type KeyVersion interface {
	GetKeyVersions(ctx context.Context, keyID uuid.UUID, pagination repo.Pagination) ([]*model.KeyVersion, int, error)
	GetLatestVersion(ctx context.Context, keyID uuid.UUID) (*model.KeyVersion, error)
	CreateVersion(
		ctx context.Context,
		keyID uuid.UUID, nativeID string, rotationTime *time.Time) (*model.KeyVersion, error)
}

type KeyVersionManager struct {
	ProviderConfigManager

	cmkAuditor      *auditor.Auditor
	landscapeConfig *config.Landscape
}

func NewKeyVersionManager(
	repo repo.Repo,
	svcRegistry serviceapi.Registry,
	tenantConfigs *TenantConfigManager,
	certManager *CertificateManager,
	cmkAuditor *auditor.Auditor,
	landscapeConfig *config.Landscape,
) *KeyVersionManager {
	return &KeyVersionManager{
		ProviderConfigManager: *NewProviderConfigManager(
			svcRegistry,
			make(map[ProviderCachedKey]*ProviderConfig),
			tenantConfigs,
			certManager,
			nil,
			repo,
		),
		cmkAuditor:      cmkAuditor,
		landscapeConfig: landscapeConfig,
	}
}

func (kvm *KeyVersionManager) GetKeyVersions(
	ctx context.Context,
	keyID uuid.UUID,
	pagination repo.Pagination,
) ([]*model.KeyVersion, int, error) {
	ck := repo.NewCompositeKey().
		Where(fmt.Sprintf("%s_%s", repo.KeyField, repo.IDField), keyID)

	return repo.ListAndCount(
		ctx,
		kvm.repo,
		pagination,
		model.KeyVersion{},
		repo.NewQuery().
			Where(repo.NewCompositeKeyGroup(ck)).
			Order(repo.OrderField{Field: repo.RotatedField, Direction: repo.Desc}).
			Order(repo.OrderField{Field: repo.CreatedField, Direction: repo.Desc}),
	)
}

// GetLatestVersion returns the latest (most recent) version for a key.
// Returns the version with the most recent RotatedAt timestamp.
// Uses created_at as a tie-breaker for deterministic ordering when multiple versions
// share the same rotated_at timestamp.
func (kvm *KeyVersionManager) GetLatestVersion(
	ctx context.Context,
	keyID uuid.UUID,
) (*model.KeyVersion, error) {
	ck := repo.NewCompositeKey().
		Where(fmt.Sprintf("%s_%s", repo.KeyField, repo.IDField), keyID)

	var version model.KeyVersion
	found, err := kvm.repo.First(
		ctx,
		&version,
		*repo.NewQuery().
			Where(repo.NewCompositeKeyGroup(ck)).
			Order(repo.OrderField{Field: repo.RotatedField, Direction: repo.Desc}).
			Order(repo.OrderField{Field: repo.CreatedField, Direction: repo.Desc}),
	)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrNoKeyVersionsFound
		}
		return nil, errs.Wrap(ErrGetKeyVersionDB, err)
	}

	if !found {
		return nil, ErrNoKeyVersionsFound
	}

	return &version, nil
}

// CreateVersion creates a new KeyVersion record.
// If a version with the same (key_id, native_id) already exists (enforced by unique
// constraint in schema), it returns the existing version instead of failing. This is
// idempotent and handles concurrent refresh operations gracefully.
func (kvm *KeyVersionManager) CreateVersion(
	ctx context.Context,
	keyID uuid.UUID,
	nativeID string,
	rotationTime *time.Time,
) (*model.KeyVersion, error) {
	// Use provided rotation time or fallback to current time
	rotatedAt := time.Now().UTC()
	if rotationTime != nil {
		rotatedAt = *rotationTime
	}

	newVersion := model.KeyVersion{
		ID:        uuid.New(),
		NativeID:  nativeID,
		KeyID:     keyID,
		RotatedAt: rotatedAt,
	}

	err := kvm.repo.Create(ctx, &newVersion)
	if err == nil {
		return &newVersion, nil
	}

	// Handle unique constraint violation
	if !errors.Is(err, repo.ErrUniqueConstraint) {
		return nil, errs.Wrap(ErrCreateKeyVersionDB, err)
	}

	// Version already exists - fetch and return it instead of failing
	ck := repo.NewCompositeKey().
		Where(fmt.Sprintf("%s_%s", repo.KeyField, repo.IDField), keyID).
		Where(repo.NativeIDField, nativeID)

	var existingVersion model.KeyVersion
	found, fetchErr := kvm.repo.First(
		ctx,
		&existingVersion,
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)),
	)
	if fetchErr != nil {
		return nil, errs.Wrap(ErrGetKeyVersionDB, fetchErr)
	}
	if !found {
		// This shouldn't happen - unique constraint failed but version not found
		return nil, errs.Wrap(ErrCreateKeyVersionDB, err)
	}

	return &existingVersion, nil
}

func (kvm *KeyVersionManager) UpdateVersions(
	ctx context.Context,
	keyID uuid.UUID,
	versions []keymanagement.KeyVersion,
) error {
	for _, k := range versions {
		if k.CreationTime != nil {
			// Insert/Update key with keystore info
			err := kvm.repo.Set(ctx, &model.KeyVersion{
				ID:        uuid.New(),
				NativeID:  k.ID,
				KeyID:     keyID,
				RotatedAt: *k.CreationTime,
				Status:    k.Status,
			}, *repo.NewQuery().
				OnConflict(repo.KeyIDField, repo.NativeIDField).
				Update(repo.RotatedField, repo.UpdatedField, repo.StatusField),
			)
			if err != nil {
				return err
			}
		} else {
			// This only runs on inserts without keystore provided time as time is always set either on the keystore or manually
			now := time.Now().UTC()
			k.CreationTime = &now
			err := kvm.repo.Create(ctx, &model.KeyVersion{
				ID:        uuid.New(),
				NativeID:  k.ID,
				KeyID:     keyID,
				RotatedAt: *k.CreationTime,
				Status:    k.Status,
			})
			if err != nil {
				return err
			}
		}
	}

	// Enforce version limits after upserting all versions
	if err := kvm.enforceVersionLimitForKey(ctx, keyID); err != nil {
		// Log warning but don't fail - versions were already saved
		log.Warn(ctx, "Failed to enforce version limit", log.ErrorAttr(err),
			slog.String("keyId", keyID.String()))
	}

	return nil
}

// enforceVersionLimitForKey enforces the configured version limit for a specific key.
// It retrieves the key's provider, checks the landscape configuration for the limit,
// and deletes the oldest non-primary versions if the count exceeds the limit.
//
// The primary version (most recent by RotatedAt) is always preserved.
// A limit of -1 (UnlimitedKeyVersions) means no eviction is performed.
//
// This method logs warnings on failures but does not return errors to avoid
// rolling back version upserts that have already succeeded.
func (kvm *KeyVersionManager) enforceVersionLimitForKey(
	ctx context.Context,
	keyID uuid.UUID,
) error {
	// 1. Get key to determine provider
	key := &model.Key{ID: keyID}
	found, err := kvm.repo.First(ctx, key, *repo.NewQuery())
	if err != nil || !found {
		return fmt.Errorf("failed to get key for version limit enforcement: %w", err)
	}

	// 2. Get max versions for this provider from landscape config, if not available, skip enforcement
	if kvm.landscapeConfig == nil {
		// No config available - skip enforcement
		log.Debug(ctx, "Landscape config not available, skipping version limit enforcement",
			slog.String("keyId", keyID.String()))
		return nil
	}

	maxVersions := kvm.landscapeConfig.GetMaxVersionsForProvider(key.Provider)
	if maxVersions == config.UnlimitedKeyVersions {
		// Unlimited versions - no eviction
		return nil
	}

	// 3. Get all current versions ordered by RotatedAt DESC (most recent first)
	allVersions, _, err := kvm.GetKeyVersions(
		ctx,
		keyID,
		repo.Pagination{Top: 10000, Skip: 0, Count: true}, // High limit to get all versions
	)
	if err != nil {
		return fmt.Errorf("failed to get versions for limit enforcement: %w", err)
	}

	// 4. Check if eviction is needed
	if len(allVersions) <= maxVersions {
		// Within limit - nothing to do
		return nil
	}

	// 5. Delete oldest versions (beyond limit)
	return kvm.deleteExcessVersions(ctx, keyID, key.Provider, allVersions, maxVersions)
}

// deleteExcessVersions removes the oldest versions beyond the configured limit.
func (kvm *KeyVersionManager) deleteExcessVersions(
	ctx context.Context,
	keyID uuid.UUID,
	provider string,
	allVersions []*model.KeyVersion,
	maxVersions int,
) error {
	// allVersions is already ordered by RotatedAt DESC, CreatedAt DESC
	// Keep first maxVersions (most recent), delete the rest
	versionsToDelete := allVersions[maxVersions:]

	log.Info(ctx, "Evicting old key versions",
		slog.String("keyId", keyID.String()),
		slog.String("provider", provider),
		slog.Int("currentCount", len(allVersions)),
		slog.Int("limit", maxVersions),
		slog.Int("toDelete", len(versionsToDelete)))

	deletedCount := 0
	for _, v := range versionsToDelete {
		_, err := kvm.repo.Delete(ctx, v, *repo.NewQuery())
		if err != nil {
			log.Error(ctx, "Failed to delete version during eviction", err,
				slog.String("versionId", v.ID.String()),
				slog.String("nativeId", v.NativeID),
				slog.String("keyId", keyID.String()))
			// Continue with others - partial eviction is better than none
			continue
		}
		deletedCount++
	}

	if deletedCount > 0 {
		log.Info(ctx, "Successfully evicted key versions",
			slog.String("keyId", keyID.String()),
			slog.Int("deletedCount", deletedCount))
	}

	return nil
}
