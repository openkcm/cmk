package manager

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
	keystoreErrs "github.com/openkcm/plugin-sdk/pkg/plugin/keystore/errors"
	commonv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/common/v1"
	keystoreopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/auditor"
	"github.com/openkcm/cmk-core/internal/constants"
	"github.com/openkcm/cmk-core/internal/errs"
	eventprocessor "github.com/openkcm/cmk-core/internal/event-processor"
	"github.com/openkcm/cmk-core/internal/log"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	"github.com/openkcm/cmk-core/utils/ptr"
)

// BYOKAction constants represent the actions that can be performed on a BYOK key
// during the import process.
type BYOKAction string

const (
	BYOKActionImportKeyMaterial BYOKAction = "IMPORT_KEY_MATERIAL"
	BYOKActionGetImportParams   BYOKAction = "GET_IMPORT_PARAMETERS"
)

type KeyManager struct {
	ProviderConfigManager

	repo             repo.Repo
	keyConfigManager *KeyConfigManager
	reconciler       *eventprocessor.CryptoReconciler
	cmkAuditor       *auditor.Auditor
}

func NewKeyManager(
	repo repo.Repo,
	catalog *plugincatalog.Catalog,
	tenantConfigs *TenantConfigManager,
	keyConfigManager *KeyConfigManager,
	certManager *CertificateManager,
	reconciler *eventprocessor.CryptoReconciler,
	cmkAuditor *auditor.Auditor,
) *KeyManager {
	return &KeyManager{
		ProviderConfigManager: ProviderConfigManager{
			catalog:       catalog,
			providers:     make(map[ProviderCachedKey]*ProviderConfig),
			tenantConfigs: tenantConfigs,
			certs:         certManager,
			keystorePool:  NewPool(repo),
		},
		repo:             repo,
		keyConfigManager: keyConfigManager,
		reconciler:       reconciler,
		cmkAuditor:       cmkAuditor,
	}
}

func (km *KeyManager) Create(
	ctx context.Context,
	key *model.Key,
) (*model.Key, error) {
	ctx = log.InjectKey(ctx, key)
	keyConfig := &model.KeyConfiguration{ID: key.KeyConfigurationID}

	_, err := km.repo.First(
		ctx,
		keyConfig,
		*repo.NewQuery(),
	)
	if err != nil {
		return nil, errs.Wrap(ErrGetConfiguration, err)
	}

	provider, err := km.GetOrInitProvider(ctx, key)
	if err != nil {
		return nil, errs.Wrap(ErrFailedToInitProvider, err)
	}

	switch key.KeyType {
	case constants.KeyTypeSystemManaged, constants.KeyTypeBYOK:
		err = km.createManagedProviderKey(ctx, key, provider)
	case constants.KeyTypeHYOK:
		err = km.registerHYOKKey(ctx, key, provider)
	default:
		return nil, ErrInvalidKeystore
	}

	if err != nil {
		return nil, err
	}

	err = km.setPrimaryIfFirstKey(ctx, key)
	if err != nil {
		return nil, errs.Wrap(ErrUpdatePrimary, err)
	}

	err = km.createKey(ctx, key)
	if err != nil {
		return nil, err
	}

	km.sendCreateAuditLog(ctx, key)

	return key, nil
}

func (km *KeyManager) Get(ctx context.Context, keyID uuid.UUID) (*model.Key, error) {
	key := &model.Key{ID: keyID}

	_, err := km.repo.First(
		ctx,
		key,
		*repo.NewQuery().Preload(repo.Preload{"KeyVersions"}),
	)
	if err != nil {
		return nil, errs.Wrap(ErrGetKeyDB, err)
	}

	switch key.KeyType {
	case constants.KeyTypeSystemManaged, constants.KeyTypeBYOK:
	case constants.KeyTypeHYOK:
		err := km.syncHYOKKeyState(ctx, key)
		if err != nil {
			return nil, err
		}
	default:
		return nil, ErrInvalidKeystore
	}

	return key, nil
}

func (km *KeyManager) GetKeys(
	ctx context.Context,
	keyConfigID *uuid.UUID,
	skip int,
	top int,
) ([]*model.Key, int, error) {
	query := repo.NewQuery().
		SetLimit(top).
		SetOffset(skip).
		Preload(repo.Preload{"KeyVersions"})

	if keyConfigID != nil {
		_, err := km.keyConfigManager.GetKeyConfigurationByID(ctx, *keyConfigID)
		if err != nil {
			return nil, 0, errs.Wrap(ErrKeyConfigurationNotFound, err)
		}

		ck := repo.NewCompositeKey().Where(repo.KeyConfigIDField, keyConfigID)
		query = query.Where(repo.NewCompositeKeyGroup(ck))
	}

	var keys []*model.Key

	count, err := km.repo.List(ctx, model.Key{}, &keys, *query)
	if err != nil {
		return nil, 0, errs.Wrap(ErrListKeysDB, err)
	}

	return keys, count, nil
}

//nolint:cyclop
func (km *KeyManager) UpdateKey(ctx context.Context, keyID uuid.UUID, keyPatch cmkapi.KeyPatch) (*model.Key, error) {
	key, err := km.Get(ctx, keyID)
	if err != nil {
		return nil, errs.Wrap(ErrGetKeyDB, err)
	}

	ctx = log.InjectKey(ctx, key)

	if key.KeyType == constants.KeyTypeHYOK && keyPatch.Enabled != nil {
		return nil, errs.Wrapf(ErrHYOKKeyActionNotAllowed, "update key state")
	}

	enablementUpdated := copyFieldsToModelKey(keyPatch, key)

	err = km.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		if keyPatch.IsPrimary != nil {
			if key.IsPrimary && !*keyPatch.IsPrimary {
				return ErrPrimaryKeyUnmark
			}

			err := km.setPrimaryKey(ctx, key)
			if err != nil {
				return errs.Wrap(ErrUpdateKeyDB, err)
			}

			key.IsPrimary = *keyPatch.IsPrimary
		}

		_, err := r.Patch(ctx, key, *repo.NewQuery().UpdateAll(true))
		if err != nil {
			return errs.Wrap(ErrUpdateKeyDB, err)
		}

		if enablementUpdated {
			if *keyPatch.Enabled {
				return km.enableKey(ctx, key)
			}

			return km.disableKey(ctx, key)
		}

		return nil
	})
	if err != nil {
		return nil, errs.Wrap(ErrUpdateKeyDB, err)
	}

	return key, nil
}

func (km *KeyManager) Delete(ctx context.Context, keyID uuid.UUID) error {
	key, err := km.Get(ctx, keyID)
	if err != nil {
		return errs.Wrap(ErrGetKeyDB, err)
	}

	if key.IsPrimary {
		exist, err := repo.HasConnectedSystems(ctx, km.repo, key.KeyConfigurationID)
		if err != nil {
			return err
		}

		if exist {
			return errs.Wrap(ErrDeleteKey, ErrConnectedSystemToKeyConfig)
		}
	}

	err = km.deleteProviderKey(ctx, key)
	if err != nil {
		return err
	}

	err = km.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		ck := repo.NewCompositeKey().
			Where(fmt.Sprintf("%s_%s", repo.KeyField, repo.IDField), keyID)

		_, err := r.Delete(
			ctx,
			&model.KeyVersion{KeyID: keyID},
			*repo.NewQuery().
				Where(repo.NewCompositeKeyGroup(ck)),
		)
		if err != nil {
			return errs.Wrap(ErrDeleteKeyDB, err)
		}

		key := &model.Key{ID: keyID}

		_, err = r.Delete(ctx, key, *repo.NewQuery())
		if err != nil {
			return errs.Wrap(ErrDeleteKeyDB, err)
		}

		return nil
	})
	if err != nil {
		return errs.Wrap(ErrDeleteKey, err)
	}

	km.sendDeleteAuditLog(ctx, key)

	return nil
}

func (km *KeyManager) UpdateVersion(ctx context.Context, keyID uuid.UUID, version int) error {
	key, err := km.Get(ctx, keyID)
	if err != nil {
		return fmt.Errorf("failed to get key: %w", err)
	}

	keyVersion := key.Version()
	keyVersion.Version = version

	_, err = km.repo.Patch(ctx, keyVersion, *repo.NewQuery())
	if err != nil {
		return fmt.Errorf("failed to update key in database: %w", err)
	}

	return nil
}

func (km *KeyManager) GetImportParams(ctx context.Context, keyID uuid.UUID) (*model.ImportParams, error) {
	key, err := km.validateBYOKKey(ctx, keyID, BYOKActionGetImportParams)
	if err != nil {
		return nil, err
	}

	if key.ImportParams != nil {
		if key.ImportParams.IsExpired() {
			return km.fetchImportParams(ctx, key)
		}

		return key.ImportParams, nil
	}

	return km.fetchImportParams(ctx, key)
}

func (km *KeyManager) ImportKeyMaterial(
	ctx context.Context,
	keyID uuid.UUID,
	wrappedKeyMaterial string,
) (*model.Key, error) {
	if wrappedKeyMaterial == "" {
		return nil, ErrEmptyKeyMaterial
	}

	_, err := base64.StdEncoding.DecodeString(wrappedKeyMaterial)
	if err != nil {
		return nil, ErrInvalidBase64KeyMaterial
	}

	key, err := km.validateBYOKKey(ctx, keyID, BYOKActionImportKeyMaterial)
	if err != nil {
		return nil, err
	}

	if key.ImportParams == nil || key.ImportParams.IsExpired() {
		return nil, ErrMissingOrExpiredImportParams
	}

	key, err = km.importProviderKeyMaterial(ctx, key, wrappedKeyMaterial)
	if err != nil {
		return nil, err
	}

	err = km.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		_, innerErr := r.Patch(ctx, key, *repo.NewQuery())
		if innerErr != nil {
			return errs.Wrap(ErrUpdateKeyDB, innerErr)
		}

		_, innerErr = r.Delete(ctx, &model.ImportParams{KeyID: keyID}, *repo.NewQuery())
		if innerErr != nil {
			return errs.Wrap(ErrDeleteImportParamsDB, innerErr)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return key, nil
}

func (km *KeyManager) SyncHYOKKeys(ctx context.Context) error {
	baseQuery := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(repo.KeyTypeField, constants.KeyTypeHYOK),
		),
	)

	return repo.ProcessInBatch[model.Key](ctx, km.repo, baseQuery, repo.DefaultLimit, func(keys []*model.Key) error {
		for _, key := range keys {
			err := km.syncHYOKKeyState(ctx, key)
			if err != nil {
				continue
			}
		}

		return nil
	})
}

func (km *KeyManager) createKey(ctx context.Context, key *model.Key) error {
	err := km.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		// Create Key
		err := r.Create(ctx, key)
		if err != nil {
			return errs.Wrap(ErrCreateKeyDB, err)
		}

		// Create KeyVersion
		if key.KeyType == constants.KeyTypeSystemManaged {
			err = r.Create(ctx, &model.KeyVersion{
				ExternalID: *key.NativeID,
				NativeID:   key.NativeID,
				KeyID:      key.ID,
				Version:    1,
				IsPrimary:  true,
			})
			if err != nil {
				return errs.Wrap(ErrCreateKeyVersionDB, err)
			}
		}

		if key.IsPrimary {
			_, err = km.repo.Patch(
				ctx,
				&model.KeyConfiguration{ID: key.KeyConfigurationID, PrimaryKeyID: &key.ID},
				*repo.NewQuery().Update(repo.PrimaryKeyIDField),
			)
			if err != nil {
				return errs.Wrap(ErrUpdateKeyConfigurationDB, err)
			}
		}

		return nil
	})
	if err != nil {
		return errs.Wrap(ErrCreateKeyDB, err)
	}

	return nil
}

func (km *KeyManager) createManagedProviderKey(
	ctx context.Context,
	key *model.Key,
	provider *ProviderConfig,
) error {
	keyResp, err := provider.Client.CreateKey(ctx, &keystoreopv1.CreateKeyRequest{
		Config: provider.Config,
		Algorithm: keystoreopv1.KeyAlgorithm(
			keystoreopv1.KeyAlgorithm_value[getPluginAlgorithm(key.Algorithm)],
		),
		Id:      ptr.PointTo(key.ID.String()),
		Region:  key.Region,
		KeyType: keystoreopv1.KeyType(keystoreopv1.KeyType_value[getPluginKeyType(key.KeyType)]),
	})
	if err != nil {
		return errs.Wrap(ErrKeyCreationFailed, err)
	}

	key.NativeID = ptr.PointTo(keyResp.GetKeyId())
	key.State = keyResp.GetStatus()

	return nil
}

func (km *KeyManager) registerHYOKKey(
	ctx context.Context,
	key *model.Key,
	provider *ProviderConfig,
) error {
	configValues := provider.Config.GetValues().GetFields()
	configValuesCopy := make(map[string]*structpb.Value, len(configValues))

	for k, v := range configValues {
		configValuesCopy[k] = v
	}

	// At this point, we assume the access data is already validated
	// in the API layer, so we can directly convert it to a struct.
	for k, v := range key.GetManagementAccessData() {
		structValue, err := structpb.NewValue(v)
		if err != nil {
			return errs.Wrapf(ErrKeyRegistration, "failed to convert access data")
		}

		configValuesCopy[k] = structValue
	}

	keyResp, err := provider.Client.GetKey(ctx, &keystoreopv1.GetKeyRequest{
		Parameters: &keystoreopv1.RequestParameters{
			KeyId: *key.NativeID,
			Config: &commonv1.KeystoreInstanceConfig{
				Values: &structpb.Struct{Fields: configValuesCopy},
			},
		},
	})
	if err != nil {
		return km.convertError(err)
	}

	if keyResp.GetAlgorithm() != keystoreopv1.KeyAlgorithm_KEY_ALGORITHM_AES256 {
		return errs.Wrapf(
			ErrUnsupportedKeyAlgorithm,
			fmt.Sprintf("%s for HYOK registration", keyResp.GetAlgorithm()))
	}

	key.Algorithm = string(cmkapi.KeyAlgorithmAES256)

	if keyResp.GetStatus() != string(cmkapi.KeyStateENABLED) {
		return errs.Wrapf(
			ErrInvalidKeyState,
			keyResp.GetStatus()+" for HYOK registration",
		)
	}

	key.State = string(cmkapi.KeyStateENABLED)

	log.Debug(
		ctx,
		"Key Register",
		slog.Group("Provider Key",
			slog.String("ID", keyResp.GetKeyId()),
			slog.String("Status", keyResp.GetStatus()),
		),
	)

	return nil
}

func (km *KeyManager) deleteProviderKey(ctx context.Context, key *model.Key) error {
	// If the key is a HYOK key, we do not delete it from the provider
	if key.KeyType == constants.KeyTypeHYOK {
		return nil
	}

	provider, err := km.GetOrInitProvider(ctx, key)
	if err != nil {
		return errs.Wrap(ErrFailedToInitProvider, err)
	}

	switch key.KeyType {
	case constants.KeyTypeSystemManaged:
		// Delete all key versions for system managed keys
		for _, kv := range key.KeyVersions {
			_, err = provider.Client.DeleteKey(ctx, &keystoreopv1.DeleteKeyRequest{
				Parameters: &keystoreopv1.RequestParameters{
					KeyId:  *kv.NativeID,
					Config: provider.Config,
				},
			})
			if err != nil {
				return errs.Wrap(ErrFailedToDeleteProvider, err)
			}
		}
	case constants.KeyTypeBYOK:
		// For BYOK keys, we delete the key itself, since BYOK keys are not versioned
		_, err = provider.Client.DeleteKey(ctx, &keystoreopv1.DeleteKeyRequest{
			Parameters: &keystoreopv1.RequestParameters{
				KeyId:  *key.NativeID,
				Config: provider.Config,
			},
		})
		if err != nil {
			return errs.Wrap(ErrFailedToDeleteProvider, err)
		}
	}

	return nil
}

func (km *KeyManager) convertError(err error) error {
	switch {
	case keystoreErrs.IsStatus(err, keystoreErrs.StatusProviderAuthenticationError):
		errWithReason := NewHYOKAuthFailedError(keystoreErrs.GetReason(err))
		return errors.Join(ErrKeyRegistration, errWithReason)
	case keystoreErrs.IsStatus(err, keystoreErrs.StatusKeyNotFound):
		return errs.Wrap(ErrKeyRegistration, ErrHYOKProviderKeyNotFound)
	default:
		return errs.Wrap(ErrKeyRegistration, err)
	}
}

func (km *KeyManager) reenableKeyVersions(ctx context.Context, key *model.Key) error {
	provider, err := km.GetOrInitProvider(ctx, key)
	if err != nil {
		return errs.Wrap(ErrFailedToInitProvider, err)
	}

	wasProviderError := false

	for _, kv := range key.KeyVersions {
		_, err = provider.Client.EnableKey(ctx, &keystoreopv1.EnableKeyRequest{
			Parameters: &keystoreopv1.RequestParameters{
				KeyId:  *kv.NativeID,
				Config: provider.Config,
			},
		})
		if err != nil {
			wasProviderError = true
		}
	}

	if wasProviderError {
		return errs.Wrap(ErrFailedToDisableProviderKey, err)
	}

	return nil
}

func (km *KeyManager) setPrimaryIfFirstKey(ctx context.Context, key *model.Key) error {
	compositeKey := repo.NewCompositeKey().Where(repo.KeyConfigIDField, key.KeyConfigurationID)
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(compositeKey))

	exist, err := km.repo.First(
		ctx,
		&model.Key{},
		*query,
	)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return err
	}

	if !exist {
		key.IsPrimary = true
	}

	return nil
}

func (km *KeyManager) getPrimaryKeys(ctx context.Context, keyConfigID *uuid.UUID) ([]*model.Key, error) {
	keys := []*model.Key{}

	_, err := km.repo.List(
		ctx,
		model.Key{},
		&keys,
		*repo.NewQuery().Where(
			repo.NewCompositeKeyGroup(
				repo.NewCompositeKey().Where(
					repo.IsPrimaryField, true).Where(
					repo.KeyConfigIDField, keyConfigID))),
	)
	if err != nil {
		return nil, errs.Wrap(ErrGetPrimaryKeyVersionDB, err)
	}

	return keys, nil
}

func (km *KeyManager) removePrimaryKeyState(ctx context.Context, keyConfigID *uuid.UUID) error {
	keys, err := km.getPrimaryKeys(ctx, keyConfigID)
	if err != nil {
		return err
	}

	for _, k := range keys {
		k.IsPrimary = false

		_, err := km.repo.Patch(
			ctx,
			k,
			*repo.NewQuery().Update(repo.IsPrimaryField))
		if err != nil {
			return errs.Wrap(ErrUpdatePrimary, err)
		}
	}

	return nil
}

func (km *KeyManager) disableKeyVersions(ctx context.Context, key *model.Key) error {
	provider, err := km.GetOrInitProvider(ctx, key)
	if err != nil {
		return errs.Wrap(ErrFailedToInitProvider, err)
	}

	wasProviderError := false

	for _, kv := range key.KeyVersions {
		_, err = provider.Client.DisableKey(ctx, &keystoreopv1.DisableKeyRequest{
			Parameters: &keystoreopv1.RequestParameters{
				KeyId:  *kv.NativeID,
				Config: provider.Config,
			},
		})
		if err != nil {
			wasProviderError = true
		}
	}

	if wasProviderError {
		return errs.Wrap(ErrFailedToDisableProviderKey, err)
	}

	return nil
}

func copyFieldsToModelKey(apiKey cmkapi.KeyPatch, dbKey *model.Key) bool {
	enablementUpdated := false

	if apiKey.Name != nil {
		dbKey.Name = *apiKey.Name
	}

	if apiKey.Description != nil {
		dbKey.Description = *apiKey.Description
	}

	if apiKey.Enabled != nil {
		if *apiKey.Enabled && dbKey.State != string(cmkapi.KeyStateENABLED) {
			dbKey.State = string(cmkapi.KeyStateENABLED)
			enablementUpdated = true
		} else if !(*apiKey.Enabled) && dbKey.State != string(cmkapi.KeyStateDISABLED) {
			dbKey.State = string(cmkapi.KeyStateDISABLED)
			enablementUpdated = true
		}
	}

	return enablementUpdated
}

func (km *KeyManager) validateBYOKKey(ctx context.Context, keyID uuid.UUID, action BYOKAction) (*model.Key, error) {
	key := &model.Key{ID: keyID}

	_, err := km.repo.First(
		ctx,
		key,
		*repo.NewQuery().Preload(repo.Preload{"ImportParams"}),
	)
	if err != nil {
		return nil, errs.Wrap(ErrGetKeyDB, err)
	}

	switch action {
	case BYOKActionGetImportParams:
		if key.KeyType != constants.KeyTypeBYOK {
			return nil, errs.Wrapf(ErrInvalidKeyTypeForImportParams,
				fmt.Sprintf("key type %s is not supported", key.KeyType))
		}

		if key.State != string(cmkapi.KeyStatePENDINGIMPORT) {
			return nil, errs.Wrapf(ErrInvalidKeyStateForImportParams,
				fmt.Sprintf("key state %s is not supported", key.State))
		}
	case BYOKActionImportKeyMaterial:
		if key.KeyType != constants.KeyTypeBYOK {
			return nil, errs.Wrapf(ErrInvalidKeyTypeForImportKeyMaterial,
				fmt.Sprintf("key type %s is not supported", key.KeyType))
		}

		if key.State != string(cmkapi.KeyStatePENDINGIMPORT) {
			return nil, errs.Wrapf(ErrInvalidKeyStateForImportKeyMaterial,
				fmt.Sprintf("key state %s is not supported", key.State))
		}
	default:
		return nil, ErrInvalidBYOKAction
	}

	return key, nil
}

func (km *KeyManager) fetchImportParams(ctx context.Context, key *model.Key) (*model.ImportParams, error) {
	provider, err := km.GetOrInitProvider(ctx, key)
	if err != nil {
		return nil, errs.Wrap(ErrFailedToInitProvider, err)
	}

	importParamsResp, err := provider.Client.GetImportParameters(ctx, &keystoreopv1.GetImportParametersRequest{
		Parameters: &keystoreopv1.RequestParameters{
			KeyId:  *key.NativeID,
			Config: provider.Config,
		},
		Algorithm: keystoreopv1.KeyAlgorithm(
			keystoreopv1.KeyAlgorithm_value[getPluginAlgorithm(key.Algorithm)],
		),
	})
	if err != nil {
		return nil, errs.Wrap(ErrGetImportParamsFromProvider, err)
	}

	importParams, err := BuildImportParams(key, importParamsResp)
	if err != nil {
		return nil, err
	}
	// Set ImportParams in db
	err = km.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		err = r.Set(ctx, importParams)
		if err != nil {
			return errs.Wrap(ErrSetImportParamsDB, err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return importParams, nil
}

func (km *KeyManager) importProviderKeyMaterial(
	ctx context.Context,
	key *model.Key,
	wrappedKeyMaterial string,
) (*model.Key, error) {
	provider, err := km.GetOrInitProvider(ctx, key)
	if err != nil {
		return nil, errs.Wrap(ErrFailedToInitProvider, err)
	}

	var providerParamsMap map[string]any

	err = json.Unmarshal(key.ImportParams.ProviderParameters, &providerParamsMap)
	if err != nil {
		return nil, err
	}

	providerParams, err := structpb.NewStruct(providerParamsMap)
	if err != nil {
		return nil, errs.Wrap(ErrCreateProtobufStruct, err)
	}

	_, err = provider.Client.ImportKeyMaterial(ctx, &keystoreopv1.ImportKeyMaterialRequest{
		Parameters: &keystoreopv1.RequestParameters{
			KeyId:  *key.NativeID,
			Config: provider.Config,
		},
		EncryptedKeyMaterial: wrappedKeyMaterial,
		ImportParameters:     providerParams,
	})
	if err != nil {
		return nil, errs.Wrap(ErrImportKeyMaterialsToProvider, err)
	}

	keyResp, err := provider.Client.GetKey(ctx, &keystoreopv1.GetKeyRequest{
		Parameters: &keystoreopv1.RequestParameters{
			KeyId:  *key.NativeID,
			Config: provider.Config,
		},
	})
	if err != nil {
		return nil, errs.Wrap(ErrGetProviderKey, err)
	}

	key.State = keyResp.GetStatus()

	return key, nil
}

// Ensures only the updated key is primary and updates the keyconfig primaryKeyID
func (km *KeyManager) setPrimaryKey(ctx context.Context, key *model.Key) error {
	if key.State != string(cmkapi.KeyStateENABLED) {
		return ErrKeyIsNotEnabled
	}

	err := km.removePrimaryKeyState(ctx, &key.KeyConfigurationID)
	if err != nil {
		return errs.Wrap(ErrUpdateKeyDB, err)
	}

	_, err = km.repo.Patch(
		ctx,
		&model.KeyConfiguration{ID: key.KeyConfigurationID, PrimaryKeyID: &key.ID},
		*repo.NewQuery().Update(repo.PrimaryKeyIDField),
	)
	if err != nil {
		return errs.Wrap(ErrUpdateKeyDB, err)
	}

	return nil
}

func (km *KeyManager) syncHYOKKeyState(ctx context.Context, key *model.Key) error {
	oldKeyState := key.State

	ctx = log.InjectKey(ctx, key)

	keyResp, err := km.getHYOKKeySync(ctx, key)
	if err != nil {
		key.State = km.getKeyStateOnSyncError(ctx, key, err)
	} else if keyResp != nil {
		// Successful case update the status in the database for the HYOK key Enabled/Disabled
		key.State = keyResp.GetStatus()
	}

	if oldKeyState == key.State {
		return nil
	}

	// Save the updated key back to the database
	err = km.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		_, txErr := r.Patch(ctx, key, *repo.NewQuery())
		if txErr != nil {
			return txErr
		}

		if key.State == string(cmkapi.KeyStateENABLED) {
			km.sendEnableAuditLog(ctx, key)

			return km.sendEnableEvent(ctx, key)
		}

		km.sendDisableAuditLog(ctx, key)

		return km.sendDisableEvent(ctx, key)
	})
	if err != nil {
		return errs.Wrap(ErrUpdateKeyDB, err)
	}

	return nil
}

func (km *KeyManager) getHYOKKeySync(ctx context.Context, key *model.Key) (*keystoreopv1.GetKeyResponse, error) {
	if key.KeyType != constants.KeyTypeHYOK {
		return nil, ErrInvalidKeyTypeForHYOKSync
	}

	provider, err := km.GetOrInitProvider(ctx, key)
	if err != nil {
		return nil, errs.Wrap(ErrFailedToInitProvider, err)
	}

	configValues := provider.Config.GetValues().GetFields()
	// At this point, we assume the access data is already validated
	// in the API layer, so we can directly convert it to a struct.
	for k, v := range key.GetManagementAccessData() {
		structValue, err := structpb.NewValue(v)
		if err != nil {
			return nil, errs.Wrapf(ErrKeyRegistration, "failed to convert access data")
		}

		configValues[k] = structValue
	}

	keyResp, err := provider.Client.GetKey(ctx, &keystoreopv1.GetKeyRequest{
		Parameters: &keystoreopv1.RequestParameters{
			KeyId:  *key.NativeID,
			Config: provider.Config,
		},
	})
	if err != nil {
		return nil, errs.Wrap(ErrGetProviderKey, err)
	}

	return keyResp, nil
}

func (km *KeyManager) sendEnableEvent(ctx context.Context, key *model.Key) error {
	if km.reconciler == nil {
		return errs.Wrapf(ErrEventSendingFailed, "reconciler is not initialized")
	}

	job, err := km.reconciler.KeyEnable(ctx, key.ID.String())
	if err != nil {
		log.Error(ctx, "Failed to send key enable event", err)
		return errs.Wrap(ErrEventSendingFailed, err)
	}

	log.Info(ctx, "Key enable event sent", slog.String("JobID", job.ID.String()))

	return nil
}

func (km *KeyManager) sendDisableEvent(ctx context.Context, key *model.Key) error {
	if km.reconciler == nil {
		return errs.Wrapf(ErrEventSendingFailed, "reconciler is not initialized")
	}

	job, err := km.reconciler.KeyDisable(ctx, key.ID.String())
	if err != nil {
		log.Error(ctx, "Failed to send key disable event", err)
		return errs.Wrap(ErrEventSendingFailed, err)
	}

	log.Info(ctx, "Key disable event sent", slog.String("JobID", job.ID.String()))

	return nil
}

func (km *KeyManager) getKeyStateOnSyncError(ctx context.Context, key *model.Key, err error) string {
	var newState string

	switch {
	// Connection issue - Unknown state
	case errs.IsAnyError(err, ErrFailedToInitProvider, ErrGetProviderKey):
		newState = string(cmkapi.KeyStateUNKNOWN)
	case errors.Is(err, ErrGetProviderKey):
		// Not successful - if access denied its Forbidden could be invalid arn or deleted key
		if strings.Contains(err.Error(), "access denied") {
			newState = string(cmkapi.KeyStateFORBIDDEN)
		} else {
			newState = string(cmkapi.KeyStateUNKNOWN)
		}
	default:
		log.Debug(ctx, "Failed to sync HYoK key", log.ErrorAttr(err))

		newState = key.State // Keep old state for now, as we cannot decide yet
	}

	return newState
}

func (km *KeyManager) sendCreateAuditLog(ctx context.Context, key *model.Key) {
	err := km.cmkAuditor.SendCmkCreateAuditLog(ctx, key.ID.String())
	if err != nil {
		log.Error(ctx, "Failed to send audit log for CMK Create", err)
	}

	log.Info(ctx, "Audit log for CMK Create sent successfully")
}

func (km *KeyManager) sendDeleteAuditLog(ctx context.Context, key *model.Key) {
	err := km.cmkAuditor.SendCmkDeleteAuditLog(ctx, key.ID.String())
	if err != nil {
		log.Error(ctx, "Failed to send audit log for CMK Delete", err)
	}

	log.Info(ctx, "Audit log for CMK Delete sent successfully")
}

func (km *KeyManager) sendDisableAuditLog(ctx context.Context, key *model.Key) {
	err := km.cmkAuditor.SendCmkDisableAuditLog(ctx, key.ID.String())
	if err != nil {
		log.Error(ctx, "Failed to send audit log for CMK Disable", err)
	}

	log.Info(ctx, "Audit log for CMK Disable sent successfully")
}

func (km *KeyManager) sendEnableAuditLog(ctx context.Context, key *model.Key) {
	err := km.cmkAuditor.SendCmkEnableAuditLog(ctx, key.ID.String())
	if err != nil {
		log.Error(ctx, "Failed to send audit log for CMK Enable", err)
	}

	log.Info(ctx, "Audit log for CMK Enable sent successfully")
}

func (km *KeyManager) enableKey(ctx context.Context, key *model.Key) error {
	err := km.reenableKeyVersions(ctx, key)
	if err != nil {
		return err
	}

	km.sendEnableAuditLog(ctx, key)

	return km.sendEnableEvent(ctx, key)
}

func (km *KeyManager) disableKey(ctx context.Context, key *model.Key) error {
	err := km.disableKeyVersions(ctx, key)
	if err != nil {
		return err
	}

	km.sendDisableAuditLog(ctx, key)

	return km.sendDisableEvent(ctx, key)
}
