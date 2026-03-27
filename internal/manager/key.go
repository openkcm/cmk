package manager

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"github.com/google/uuid"
	"github.com/openkcm/orbital"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/key/transformer"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/common"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/utils/ptr"
)

// BYOKAction constants represent the actions that can be performed on a BYOK key
// during the import process.
type BYOKAction string

const (
	BYOKActionImportKeyMaterial BYOKAction = "IMPORT_KEY_MATERIAL"
	BYOKActionGetImportParams   BYOKAction = "GET_IMPORT_PARAMETERS"
	IsEditableCryptoAccess      string     = "isEditable"
)

var UnavailableKeyStates = []string{
	string(cmkapi.KeyStatePENDINGDELETION),
	string(cmkapi.KeyStateDELETED),
	string(cmkapi.KeyStateFORBIDDEN),
	string(cmkapi.KeyStateUNKNOWN),
}

func IsUnavailableKeyState(state string) bool {
	return slices.Contains(UnavailableKeyStates, state)
}

type KeyManager struct {
	ProviderConfigManager

	repo             repo.Repo
	keyConfigManager *KeyConfigManager
	user             User
	eventFactory     *eventprocessor.EventFactory
	cmkAuditor       *auditor.Auditor
}

func NewKeyManager(
	repo repo.Repo,
	svcRegistry *cmkpluginregistry.Registry,
	tenantConfigs *TenantConfigManager,
	keyConfigManager *KeyConfigManager,
	user User,
	certManager *CertificateManager,
	eventFactory *eventprocessor.EventFactory,
	cmkAuditor *auditor.Auditor,
) *KeyManager {
	return &KeyManager{
		ProviderConfigManager: *NewProviderConfigManager(
			svcRegistry,
			make(map[ProviderCachedKey]*ProviderConfig),
			tenantConfigs,
			certManager,
			NewPool(repo),
			repo,
		),
		repo:             repo,
		keyConfigManager: keyConfigManager,
		user:             user,
		eventFactory:     eventFactory,
		cmkAuditor:       cmkAuditor,
	}
}

func (km *KeyManager) Create(
	ctx context.Context,
	key *model.Key,
) (*model.Key, error) {
	ctx = model.LogInjectKey(ctx, key)
	_, err := km.user.HasKeyAccess(ctx, authz.APIActionCreate, key.KeyConfigurationID)
	if err != nil {
		return nil, err
	}

	keyConfig := &model.KeyConfiguration{ID: key.KeyConfigurationID}

	_, err = km.repo.First(
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

	err = km.repo.Transaction(ctx, func(ctx context.Context) error {
		err := km.setPrimaryIfFirstKey(ctx, key)
		if err != nil {
			return errs.Wrap(ErrUpdatePrimary, err)
		}

		return km.createKey(ctx, key)
	})
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

	_, err = km.user.HasKeyAccess(ctx, authz.APIActionRead, key.KeyConfigurationID)
	if err != nil {
		return nil, err
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

	err = km.setEditableStatus(ctx, key)
	if err != nil {
		return nil, err
	}

	return key, nil
}

func (km *KeyManager) GetKeys(
	ctx context.Context,
	keyConfigID uuid.UUID,
	pagination repo.Pagination,
) ([]*model.Key, int, error) {
	query := repo.NewQuery().
		Preload(repo.Preload{"KeyVersions"})

	_, err := km.user.HasKeyAccess(ctx, authz.APIActionRead, keyConfigID)
	if err != nil {
		return nil, 0, err
	}

	_, err = km.keyConfigManager.GetKeyConfigurationByID(ctx, keyConfigID)
	if err != nil {
		return nil, 0, errs.Wrap(ErrKeyConfigurationNotFound, err)
	}

	ck := repo.NewCompositeKey().Where(fmt.Sprintf("%s.%s", model.Key{}.TableName(), repo.KeyConfigIDField), keyConfigID)
	query = query.Where(repo.NewCompositeKeyGroup(ck))

	return repo.ListAndCount(ctx, km.repo, pagination, model.Key{}, query)
}

func (km *KeyManager) UpdateKey(ctx context.Context, keyID uuid.UUID, keyPatch cmkapi.KeyPatch) (*model.Key, error) {
	if isManagementDetailsUpdate(keyPatch) {
		return nil, ErrManagementDetailsUpdate
	}

	key, err := km.Get(ctx, keyID)
	if err != nil {
		return nil, errs.Wrap(ErrGetKeyDB, err)
	}

	ctx = model.LogInjectKey(ctx, key)

	err = km.handleCryptoDetailsUpdate(ctx, keyPatch, key)
	if err != nil {
		return nil, errs.Wrap(ErrCryptoDetailsUpdate, err)
	}

	if key.KeyType == constants.KeyTypeHYOK && keyPatch.Enabled != nil {
		return nil, errs.Wrapf(ErrHYOKKeyActionNotAllowed, "update key state")
	}

	enablementUpdated := copyFieldsToModelKey(keyPatch, key)

	err = km.repo.Transaction(ctx, func(ctx context.Context) error {
		_, err := km.repo.Patch(ctx, key, *repo.NewQuery().UpdateAll(true))
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

	isPrimary, err := repo.IsPrimaryKey(ctx, km.repo, key)
	if err != nil {
		return err
	}

	if isPrimary {
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

	err = km.repo.Transaction(ctx, func(ctx context.Context) error {
		ck := repo.NewCompositeKey().
			Where(fmt.Sprintf("%s_%s", repo.KeyField, repo.IDField), keyID)

		_, err := km.repo.Delete(
			ctx,
			&model.KeyVersion{KeyID: keyID},
			*repo.NewQuery().
				Where(repo.NewCompositeKeyGroup(ck)),
		)
		if err != nil {
			return errs.Wrap(ErrDeleteKeyDB, err)
		}

		key := &model.Key{ID: keyID}

		_, err = km.repo.Delete(ctx, key, *repo.NewQuery())
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

	err = km.repo.Transaction(ctx, func(ctx context.Context) error {
		_, innerErr := km.repo.Patch(ctx, key, *repo.NewQuery())
		if innerErr != nil {
			return errs.Wrap(ErrUpdateKeyDB, innerErr)
		}

		_, innerErr = km.repo.Delete(ctx, &model.ImportParams{KeyID: keyID}, *repo.NewQuery())
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

func (km *KeyManager) Detach(ctx context.Context, key *model.Key) error {
	return km.repo.Transaction(ctx, func(ctx context.Context) error {
		key.State = string(cmkapi.KeyStateDETACHING)

		_, err := km.repo.Patch(ctx, key, *repo.NewQuery())
		if err != nil {
			return err
		}

		err = km.sendDetachEvent(ctx, key)
		if err != nil {
			return err
		}
		return nil
	})
}

func (km *KeyManager) setEditableStatus(ctx context.Context, key *model.Key) error {
	cryptoData := key.GetCryptoAccessData()
	if cryptoData == nil {
		return nil
	}

	isPrimary, err := repo.IsPrimaryKey(ctx, km.repo, key)
	if err != nil {
		return err
	}

	if !isPrimary {
		for region := range cryptoData {
			key.EditableRegions[region] = true
		}
		return nil
	}

	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(repo.KeyConfigIDField, key.KeyConfigurationID),
		),
	)

	return repo.ProcessInBatch(ctx, km.repo, query, repo.DefaultLimit, func(systems []*model.System) error {
		for _, s := range systems {
			key.EditableRegions[s.Region] = s.Status == cmkapi.SystemStatusFAILED
		}

		return nil
	})
}

func isManagementDetailsUpdate(keyPatch cmkapi.KeyPatch) bool {
	patchAccessDetails := keyPatch.AccessDetails
	return patchAccessDetails != nil && patchAccessDetails.Management != nil
}

func (km *KeyManager) handleCryptoDetailsUpdate(
	ctx context.Context,
	keyPatch cmkapi.KeyPatch,
	key *model.Key,
) error {
	patchAccessDetails := keyPatch.AccessDetails

	if patchAccessDetails == nil || patchAccessDetails.Crypto == nil {
		return nil
	}

	providerTransformer, err := transformer.NewPluginProviderTransformer(km.svcRegistry, key.Provider)
	if err != nil {
		return err
	}

	keyPatch.AccessDetails.Management = ptr.PointTo(key.GetManagementAccessData())

	err = providerTransformer.ValidateKeyAccessData(ctx, keyPatch.AccessDetails)
	if err != nil {
		return errs.Wrap(ErrBadCryptoRegionData, err)
	}

	keyCryptoData := key.GetCryptoAccessData()
	for region, regionValues := range *patchAccessDetails.Crypto {
		editable, exist := key.EditableRegions[region]
		if !editable && exist {
			return ErrNonEditableCryptoRegionUpdate
		}
		keyCryptoData[region] = regionValues
	}

	bytes, err := json.Marshal(keyCryptoData)
	if err != nil {
		return err
	}

	key.CryptoAccessData = bytes

	return nil
}

func (km *KeyManager) createKey(ctx context.Context, key *model.Key) error {
	err := km.repo.Transaction(ctx, func(ctx context.Context) error {
		// Create Key
		err := km.repo.Create(ctx, key)
		if err != nil {
			return errs.Wrap(ErrCreateKeyDB, err)
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
	keyResp, err := provider.Client.CreateKey(ctx, &keymanagement.CreateKeyRequest{
		Config:       common.KeystoreConfig{Values: provider.Config.Values},
		KeyAlgorithm: convertToAPIKeyAlgorithm(key.Algorithm),
		ID:           ptr.PointTo(key.ID.String()),
		Region:       key.Region,
		KeyType:      convertToAPIKeyType(key.KeyType),
	})
	if err != nil {
		return errs.Wrap(ErrKeyCreationFailed, err)
	}

	key.NativeID = ptr.PointTo(keyResp.KeyID)
	key.State = keyResp.Status

	return nil
}

func (km *KeyManager) registerHYOKKey(
	ctx context.Context,
	key *model.Key,
	provider *ProviderConfig,
) error {
	configValues := mergeProviderConfigValuesWithKeyAccessData(provider, key)

	keyResp, err := provider.Client.GetKey(ctx, &keymanagement.GetKeyRequest{
		Parameters: keymanagement.RequestParameters{
			Config: common.KeystoreConfig{Values: configValues},
			KeyID:  *key.NativeID,
		},
	})
	if err != nil {
		return errs.Wrap(ErrKeyRegistration, err)
	}

	if keyResp.KeyAlgorithm != keymanagement.AES256 {
		return errs.Wrapf(
			ErrUnsupportedKeyAlgorithm,
			fmt.Sprintf("%v for HYOK registration", keyResp.KeyAlgorithm))
	}

	key.Algorithm = string(cmkapi.KeyAlgorithmAES256)

	if keyResp.Status != string(cmkapi.KeyStateENABLED) {
		return errs.Wrapf(
			ErrInvalidKeyState,
			keyResp.Status+" for HYOK registration",
		)
	}

	key.State = string(cmkapi.KeyStateENABLED)

	log.Debug(
		ctx,
		"Key Register",
		slog.Group("Provider Key",
			slog.String("id", keyResp.KeyID),
			slog.String("status", keyResp.Status),
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
			_, err = provider.Client.DeleteKey(ctx, &keymanagement.DeleteKeyRequest{
				Parameters: keymanagement.RequestParameters{
					Config: common.KeystoreConfig{Values: maps.Clone(provider.Config.Values)},
					KeyID:  *kv.NativeID,
				},
			})
			if err != nil {
				return errs.Wrap(ErrFailedToDeleteProvider, err)
			}
		}
	case constants.KeyTypeBYOK:
		// For BYOK keys, we delete the key itself, since BYOK keys are not versioned
		_, err = provider.Client.DeleteKey(ctx, &keymanagement.DeleteKeyRequest{
			Parameters: keymanagement.RequestParameters{
				Config: common.KeystoreConfig{Values: maps.Clone(provider.Config.Values)},
				KeyID:  *key.NativeID,
			},
		})
		if err != nil {
			return errs.Wrap(ErrFailedToDeleteProvider, err)
		}
	}

	return nil
}

func (km *KeyManager) reenableKeyVersions(ctx context.Context, key *model.Key) error {
	provider, err := km.GetOrInitProvider(ctx, key)
	if err != nil {
		return errs.Wrap(ErrFailedToInitProvider, err)
	}

	wasProviderError := false

	for _, kv := range key.KeyVersions {
		_, err = provider.Client.EnableKey(ctx, &keymanagement.EnableKeyRequest{
			Parameters: keymanagement.RequestParameters{
				Config: common.KeystoreConfig{Values: maps.Clone(provider.Config.Values)},
				KeyID:  *kv.NativeID,
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

	// Update keyconfig primaryKey
	if !exist {
		if key.State == string(cmkapi.KeyStateDISABLED) {
			return ErrKeyIsNotEnabled
		}
		keyConfig := &model.KeyConfiguration{
			ID:           key.KeyConfigurationID,
			PrimaryKeyID: &key.ID,
		}
		_, err := km.repo.Patch(ctx, keyConfig, *repo.NewQuery())
		if err != nil {
			return err
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
		_, err = provider.Client.DisableKey(ctx, &keymanagement.DisableKeyRequest{
			Parameters: keymanagement.RequestParameters{
				Config: common.KeystoreConfig{Values: maps.Clone(provider.Config.Values)},
				KeyID:  *kv.NativeID,
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

func mergeProviderConfigValuesWithKeyAccessData(
	provider *ProviderConfig,
	key *model.Key,
) map[string]any {
	// Start with the provider config values
	configValues := provider.Config.Values

	// Create a copy to avoid modifying the original
	merged := make(map[string]any, len(configValues)+len(key.GetManagementAccessData()))
	maps.Copy(merged, configValues)

	// At this point, we assume the access data is already validated
	// in the API layer, so we can directly merge it.
	maps.Copy(merged, key.GetManagementAccessData())

	return merged
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

	importParamsResp, err := provider.Client.GetImportParameters(ctx, &keymanagement.GetImportParametersRequest{
		Parameters: keymanagement.RequestParameters{
			Config: common.KeystoreConfig{Values: maps.Clone(provider.Config.Values)},
			KeyID:  *key.NativeID,
		},
		KeyAlgorithm: convertToAPIKeyAlgorithm(key.Algorithm),
	})
	if err != nil {
		return nil, errs.Wrap(ErrGetImportParamsFromProvider, err)
	}

	importParams, err := BuildImportParamsFromAPI(key, importParamsResp)
	if err != nil {
		return nil, err
	}
	// Set ImportParams in DB
	err = km.repo.Transaction(ctx, func(ctx context.Context) error {
		err = km.repo.Set(ctx, importParams)
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

	_, err = provider.Client.ImportKeyMaterial(ctx, &keymanagement.ImportKeyMaterialRequest{
		Parameters: keymanagement.RequestParameters{
			Config: common.KeystoreConfig{Values: maps.Clone(provider.Config.Values)},
			KeyID:  *key.NativeID,
		},
		EncryptedKeyMaterial: wrappedKeyMaterial,
		ImportParameters:     providerParamsMap,
	})
	if err != nil {
		return nil, errs.Wrap(ErrImportKeyMaterialsToProvider, err)
	}

	keyResp, err := provider.Client.GetKey(ctx, &keymanagement.GetKeyRequest{
		Parameters: keymanagement.RequestParameters{
			Config: common.KeystoreConfig{Values: maps.Clone(provider.Config.Values)},
			KeyID:  *key.NativeID,
		},
	})
	if err != nil {
		return nil, errs.Wrap(ErrGetProviderKey, err)
	}

	key.State = keyResp.Status

	return key, nil
}

func (km *KeyManager) syncHYOKKeyState(ctx context.Context, key *model.Key) error {
	oldKeyState := key.State

	ctx = model.LogInjectKey(ctx, key)

	keyResp, err := km.getHYOKKeySync(ctx, key)
	if err != nil {
		key.State = km.getKeyStateOnSyncError(ctx, key, err)
		km.sendUnavailableAuditLog(ctx, key)
	} else if keyResp != nil {
		// Successful case update the status in the database for the HYOK key Enabled/Disabled
		key.State = keyResp.Status
	}

	if oldKeyState == key.State {
		return nil
	}

	// Save the updated key back to the database
	err = km.repo.Transaction(ctx, func(ctx context.Context) error {
		_, txErr := km.repo.Patch(ctx, key, *repo.NewQuery())
		if txErr != nil {
			return txErr
		}

		return km.handleKeyStateTransition(ctx, key, oldKeyState)
	})
	if err != nil {
		return errs.Wrap(ErrUpdateKeyDB, err)
	}

	return nil
}

func (km *KeyManager) handleKeyStateTransition(ctx context.Context, key *model.Key, oldKeyState string) error {
	switch key.State {
	case string(cmkapi.KeyStateENABLED):
		if IsUnavailableKeyState(oldKeyState) {
			km.sendAvailableAuditLog(ctx, key)
		} else {
			km.sendEnableAuditLog(ctx, key)
		}

		return km.sendEnableEvent(ctx, key)
	case string(cmkapi.KeyStatePENDINGDELETION):
		km.sendUnavailableAuditLog(ctx, key)
		return nil
	case string(cmkapi.KeyStateDISABLED):
		// When transitioning from unavailable states (DELETED, PENDING_DELETION, UNKNOWN, FORBIDDEN)
		// to DISABLED, we send AvailableAuditLog because DISABLED is considered an available state.
		// The key is still accessible despite being disabled.
		//
		// Key availability states:
		// - Available: ENABLED, DISABLED
		// - Unavailable: DELETED, PENDING_DELETION, UNKNOWN, FORBIDDEN
		//
		// Common scenarios:
		// 1. Customer deletes key on provider → key becomes PENDING_DELETION (unavailable)
		//    Customer cancels deletion → key transitions to DISABLED (available again)
		// 2. Customer removes access permissions → key becomes FORBIDDEN (unavailable)
		//    Customer restores permissions → key transitions to DISABLED (available again)
		// 3. Provider connection issues → key becomes UNKNOWN (unavailable)
		//    Connection restored → key transitions to DISABLED (available again)
		if IsUnavailableKeyState(oldKeyState) {
			km.sendAvailableAuditLog(ctx, key)
		} else {
			km.sendDisableAuditLog(ctx, key)
		}

		return km.sendDisableEvent(ctx, key)
	default:
		return nil
	}
}

func (km *KeyManager) getHYOKKeySync(ctx context.Context, key *model.Key) (*keymanagement.GetKeyResponse, error) {
	if key.KeyType != constants.KeyTypeHYOK {
		return nil, ErrInvalidKeyTypeForHYOKSync
	}

	provider, err := km.GetOrInitProvider(ctx, key)
	if err != nil {
		return nil, errs.Wrap(ErrFailedToInitProvider, err)
	}

	configValues := mergeProviderConfigValuesWithKeyAccessData(provider, key)

	keyResp, err := provider.Client.GetKey(ctx, &keymanagement.GetKeyRequest{
		Parameters: keymanagement.RequestParameters{
			Config: common.KeystoreConfig{Values: configValues},
			KeyID:  *key.NativeID,
		},
	})
	if err != nil {
		return nil, errs.Wrap(ErrGetProviderKey, err)
	}

	return keyResp, nil
}

func (km *KeyManager) sendEnableEvent(ctx context.Context, key *model.Key) error {
	return km.eventFactory.SendEvent(ctx, eventprocessor.Event{
		Name: eventprocessor.JobTypeKeyEnable.String(),
		Event: func(ctx context.Context) (orbital.Job, error) {
			job, err := km.eventFactory.KeyEnable(ctx, key.ID.String())
			if errors.Is(err, orbital.ErrJobAlreadyExists) {
				log.Info(ctx, "Key enable event already exists", slog.String("jobId", job.ID.String()))
				return job, nil
			}

			return job, err
		},
	})
}

func (km *KeyManager) sendDisableEvent(ctx context.Context, key *model.Key) error {
	return km.eventFactory.SendEvent(ctx, eventprocessor.Event{
		Name: eventprocessor.JobTypeKeyDisable.String(),
		Event: func(ctx context.Context) (orbital.Job, error) {
			job, err := km.eventFactory.KeyDisable(ctx, key.ID.String())
			if errors.Is(err, orbital.ErrJobAlreadyExists) {
				log.Info(ctx, "Key disable event already exists", slog.String("jobId", job.ID.String()))
				return job, nil
			}

			return job, err
		},
	})
}

func (km *KeyManager) sendDetachEvent(ctx context.Context, key *model.Key) error {
	return km.eventFactory.SendEvent(ctx, eventprocessor.Event{
		Name: eventprocessor.JobTypeKeyDetach.String(),
		Event: func(ctx context.Context) (orbital.Job, error) {
			job, err := km.eventFactory.KeyDetach(ctx, key.ID.String())
			if errors.Is(err, orbital.ErrJobAlreadyExists) {
				log.Info(ctx, "Key detach event already exists", slog.String("jobId", job.ID.String()))
				return job, nil
			}

			return job, err
		},
	})
}

func (km *KeyManager) getKeyStateOnSyncError(ctx context.Context, key *model.Key, err error) string {
	var newState string

	switch {
	case errors.Is(err, keymanagement.ErrProviderAuthenticationFailed):
		newState = string(cmkapi.KeyStateFORBIDDEN)
	case errors.Is(err, keymanagement.ErrHYOKKeyNotFound):
		newState = string(cmkapi.KeyStateDELETED)
	case errs.IsAnyError(err, ErrFailedToInitProvider, ErrGetProviderKey):
		newState = string(cmkapi.KeyStateUNKNOWN)
	default:
		log.Debug(ctx, "Failed to sync HYOK key", log.ErrorAttr(err))

		newState = key.State // Keep old state for now, as we cannot decide yet
	}

	return newState
}

func (km *KeyManager) sendCreateAuditLog(ctx context.Context, key *model.Key) {
	err := km.cmkAuditor.SendCmkCreateAuditLog(ctx, key.ID.String())
	if err != nil {
		log.Error(ctx, "Failed to send audit log for CMK Create", err)
		return
	}

	log.Info(ctx, "Audit log for CMK Create sent successfully")
}

func (km *KeyManager) sendDeleteAuditLog(ctx context.Context, key *model.Key) {
	err := km.cmkAuditor.SendCmkDeleteAuditLog(ctx, key.ID.String())
	if err != nil {
		log.Error(ctx, "Failed to send audit log for CMK Delete", err)
		return
	}

	log.Info(ctx, "Audit log for CMK Delete sent successfully")
}

func (km *KeyManager) sendDisableAuditLog(ctx context.Context, key *model.Key) {
	err := km.cmkAuditor.SendCmkDisableAuditLog(ctx, key.ID.String())
	if err != nil {
		log.Error(ctx, "Failed to send audit log for CMK Disable", err)
		return
	}

	log.Info(ctx, "Audit log for CMK Disable sent successfully")
}

func (km *KeyManager) sendEnableAuditLog(ctx context.Context, key *model.Key) {
	err := km.cmkAuditor.SendCmkEnableAuditLog(ctx, key.ID.String())
	if err != nil {
		log.Error(ctx, "Failed to send audit log for CMK Enable", err)
		return
	}

	log.Info(ctx, "Audit log for CMK Enable sent successfully")
}

func (km *KeyManager) sendAvailableAuditLog(ctx context.Context, key *model.Key) {
	err := km.cmkAuditor.SendCmkAvailableAuditLog(ctx, key.ID.String())
	if err != nil {
		log.Error(ctx, "Failed to send audit log for CMK Available", err)
		return
	}

	log.Info(ctx, "Audit log for CMK Available sent successfully")
}

func (km *KeyManager) sendUnavailableAuditLog(ctx context.Context, key *model.Key) {
	err := km.cmkAuditor.SendCmkUnavailableAuditLog(ctx, key.ID.String())
	if err != nil {
		log.Error(ctx, "Failed to send audit log for CMK Unavailable", err)
		return
	}

	log.Info(ctx, "Audit log for CMK Unavailable sent successfully")
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

func convertToAPIKeyAlgorithm(alg string) keymanagement.KeyAlgorithm {
	switch alg {
	case string(cmkapi.KeyAlgorithmAES256):
		return keymanagement.AES256
	case string(cmkapi.KeyAlgorithmRSA3072):
		return keymanagement.RSA3072
	case string(cmkapi.KeyAlgorithmRSA4096):
		return keymanagement.RSA4096
	default:
		return keymanagement.UnspecifiedKeyAlgorithm
	}
}

func convertToAPIKeyType(keyType string) keymanagement.KeyType {
	switch keyType {
	case constants.KeyTypeSystemManaged:
		return keymanagement.SystemManaged
	case constants.KeyTypeBYOK:
		return keymanagement.BYOK
	case constants.KeyTypeHYOK:
		return keymanagement.HYOK
	default:
		return keymanagement.UnspecifiedKeyType
	}
}
