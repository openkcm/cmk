package manager

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/google/uuid"

	keystoreopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	cmkplugincatalog "github.com/openkcm/cmk/internal/plugincatalog"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/utils/ptr"
)

type KeyVersion interface {
	GetKeyVersions(ctx context.Context, keyID uuid.UUID, skip int, top int) ([]model.KeyVersion, int, error)
	CreateKeyVersion(ctx context.Context, keyID uuid.UUID, nativeID *string) (*model.KeyVersion, error)
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
	catalog *cmkplugincatalog.Registry,
	tenantConfigs *TenantConfigManager,
	certManager *CertificateManager,
	cmkAuditor *auditor.Auditor,
) *KeyVersionManager {
	return &KeyVersionManager{
		ProviderConfigManager: ProviderConfigManager{
			catalog:       catalog,
			providers:     make(map[ProviderCachedKey]*ProviderConfig),
			tenantConfigs: tenantConfigs,
			repo:          repo,
			certs:         certManager,
		},
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
		repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)),
	)
}

func (kvm *KeyVersionManager) CreateKeyVersion(
	ctx context.Context,
	keyID uuid.UUID,
	nativeID *string,
) (*model.KeyVersion, error) {
	key := &model.Key{ID: keyID}

	_, err := kvm.repo.First(
		ctx,
		key,
		*repo.NewQuery().Preload(repo.Preload{"KeyVersions"}),
	)
	if err != nil {
		return nil, errs.Wrap(ErrGetKeyDB, err)
	}

	switch key.KeyType {
	case string(cmkapi.KeyTypeBYOK):
		return nil, ErrRotateBYOKKey
	case string(cmkapi.KeyTypeHYOK):
		if nativeID == nil {
			return nil, ErrNoBodyForCustomerHeldDB
		}
	default:
		if nativeID != nil {
			// Rotate scenario
			return nil, ErrBodyForNoCustomerHeldDB
		}
	}

	keyVersion, err := kvm.AddKeyVersion(
		ctx,
		*key,
		nativeID,
	)
	if err != nil || keyVersion == nil {
		return nil, ErrCreateKeyVersionDB
	}

	kvm.sendRotateAuditLog(ctx, key)

	return keyVersion, err
}

// AddKeyVersion creates a new key version in repository and client provider.
func (kvm *KeyVersionManager) AddKeyVersion(ctx context.Context,
	key model.Key,
	_ *string,
) (*model.KeyVersion, error) {
	nativeID, err := kvm.createKeyProvider(ctx, &key)
	if err != nil {
		return nil, err
	}

	return kvm.createDBKeyVersion(ctx, &key, nativeID)
}

func (kvm *KeyVersionManager) GetByKeyIDAndByNumber(
	ctx context.Context,
	keyID uuid.UUID,
	keyVersionNumber string,
) (*model.KeyVersion, error) {
	keyVersion := &model.KeyVersion{}

	if strings.ToLower(keyVersionNumber) == "latest" {
		ck := repo.NewCompositeKey().
			Where(repo.IsPrimaryField, true).
			Where(fmt.Sprintf("%s_%s", repo.KeyField, repo.IDField), keyID)

		_, err := kvm.repo.First(
			ctx,
			keyVersion,
			*repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)),
		)
		if err != nil {
			return nil, errs.Wrap(ErrGetPrimaryKeyVersionDB, err)
		}

		return keyVersion, nil
	}

	_, err := strconv.Atoi(keyVersionNumber)
	if err != nil {
		return nil, errs.Wrap(ErrInvalidKeyVersionNumber, err)
	}

	ck := repo.NewCompositeKey().
		Where(repo.VersionField, keyVersionNumber).
		Where(fmt.Sprintf("%s_%s", repo.KeyField, repo.IDField), keyID)

	_, err = kvm.repo.First(
		ctx,
		keyVersion,
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
	if err != nil {
		return nil, errs.Wrap(ErrGetKeyVersionDB, err)
	}

	return keyVersion, nil
}

func (kvm *KeyVersionManager) incrementVersion(key *model.Key) int {
	currentVersion := key.Version()

	if currentVersion == nil {
		return 0
	}

	return currentVersion.Version + 1
}

func (kvm *KeyVersionManager) createKeyProvider(ctx context.Context, key *model.Key) (string, error) {
	provider, err := kvm.GetOrInitProvider(ctx, key)
	if err != nil {
		return "", errs.Wrap(ErrFailedToInitProvider, err)
	}

	// create key in provider
	keyResponse, err := provider.Client.CreateKey(ctx, &keystoreopv1.CreateKeyRequest{
		Config: provider.Config,
		Algorithm: keystoreopv1.KeyAlgorithm(
			keystoreopv1.KeyAlgorithm_value[getPluginAlgorithm(key.Algorithm)],
		),
		Id:      ptr.PointTo(key.ID.String()),
		Region:  key.Region,
		KeyType: keystoreopv1.KeyType(keystoreopv1.KeyType_value[getPluginKeyType(key.KeyType)]),
	})
	if err != nil {
		return "", errs.Wrap(ErrKeyCreationFailed, err)
	}

	nativeID := keyResponse.GetKeyId()

	return nativeID, nil
}

func (kvm *KeyVersionManager) createDBKeyVersion(
	ctx context.Context,
	key *model.Key,
	nativeID string,
) (*model.KeyVersion, error) {
	newKeyVersion := model.KeyVersion{
		ExternalID: uuid.New().String(),
		NativeID:   &nativeID,
		KeyID:      key.ID,
		Version:    kvm.incrementVersion(key),
		IsPrimary:  true,
	}

	err := kvm.repo.Transaction(ctx, func(ctx context.Context) error {
		err := kvm.disablePrimaryVersions(ctx, key)
		if err != nil {
			return err
		}

		err = kvm.repo.Create(ctx, &newKeyVersion)
		if err != nil {
			return errs.Wrap(ErrCreateKeyVersionDB, err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &newKeyVersion, nil
}

func (kvm *KeyVersionManager) disablePrimaryVersions(ctx context.Context, key *model.Key) error {
	var oldKeyVersions []model.KeyVersion

	ck := repo.NewCompositeKey().
		Where(fmt.Sprintf("%s_%s", repo.KeyField, repo.IDField), key.ID)

	err := kvm.repo.List(
		ctx,
		model.KeyVersion{},
		&oldKeyVersions,
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)),
	)
	if err != nil {
		return errs.Wrap(ErrListKeyVersionsDB, err)
	}

	for _, oldKeyVersion := range oldKeyVersions {
		if oldKeyVersion.IsPrimary {
			oldKeyVersion.IsPrimary = false

			_, err := kvm.repo.Patch(ctx, &oldKeyVersion, *repo.NewQuery().UpdateAll(true))
			if err != nil {
				return errs.Wrap(ErrUpdateKeyVersionDB, err)
			}
		}
	}

	return nil
}

func (kvm *KeyVersionManager) sendRotateAuditLog(ctx context.Context, key *model.Key) {
	err := kvm.cmkAuditor.SendCmkRotateAuditLog(ctx, key.ID.String())
	if err != nil {
		log.Error(ctx, "Failed to send Audit log for CMK Rotate", err)
	}

	log.Info(ctx, "Audit log for CMK Rotate sent successfully", slog.String("keyID", key.ID.String()))
}
