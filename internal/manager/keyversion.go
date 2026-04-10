package manager

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo"
)

type KeyVersion interface {
	GetKeyVersions(ctx context.Context, keyID uuid.UUID, skip int, top int) ([]model.KeyVersion, int, error)
	GetKeyVersionByNumber(ctx context.Context, keyID uuid.UUID, version string) (*model.KeyVersion, error)
	UpdateKeyVersion(
		ctx context.Context,
		keyID uuid.UUID,
		version string,
		enabled *bool,
	) error
}

type KeyVersionManager struct {
	ProviderConfigManager

	cmkAuditor *auditor.Auditor
}

func NewKeyVersionManager(
	repo repo.Repo,
	svcRegistry *cmkpluginregistry.Registry,
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
			Order(repo.OrderField{Field: repo.RotatedField, Direction: repo.Desc}),
	)
}

// CreateVersion creates a new KeyVersion record
func (kvm *KeyVersionManager) CreateVersion(
	ctx context.Context,
	keyID uuid.UUID,
	nativeID string,
	rotationTime *time.Time,
) (*model.KeyVersion, error) {
	newVersion := model.KeyVersion{
		ID:        uuid.New(),
		NativeID:  nativeID,
		KeyID:     keyID,
		RotatedAt: rotationTime,
	}

	err := kvm.repo.Create(ctx, &newVersion)
	if err != nil {
		return nil, errs.Wrap(ErrCreateKeyVersionDB, err)
	}

	return &newVersion, nil
}
