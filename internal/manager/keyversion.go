package manager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
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

	cmkAuditor *auditor.Auditor
}

func NewKeyVersionManager(
	repo repo.Repo,
	svcRegistry serviceapi.Registry,
	tenantConfigs *TenantConfigManager,
	certManager *CertificateManager,
	cmkAuditor *auditor.Auditor,
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
		cmkAuditor: cmkAuditor,
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
