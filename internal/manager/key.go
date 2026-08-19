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
	"time"

	"github.com/avast/retry-go/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/openkcm/orbital"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/key/transformer"
	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/common"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
	"github.com/openkcm/cmk/internal/repo"
	asyncUtils "github.com/openkcm/cmk/utils/async"
)

// BYOKAction constants represent the actions that can be performed on a BYOK key
// during the import process.
type BYOKAction string

const (
	BYOKActionImportKeyMaterial BYOKAction = "IMPORT_KEY_MATERIAL"
	BYOKActionGetImportParams   BYOKAction = "GET_IMPORT_PARAMETERS"
	IsEditableCryptoAccess      string     = "isEditable"

	// createKeyRetryAttempts is the number of attempts for key creation, including the initial attempt.
	// Covers keystore provider authorization propagation delay (~1-2 min) after lazy role provisioning.
	// With 15s base, 30s cap, and BackOffDelay: 15 + 30 + 30 + 30 = 105s total wait.
	createKeyRetryAttempts = 5
)

// createKeyRetryDelay and createKeyMaxDelay control the backoff for key creation retries.
// They are vars (not consts) so tests can override them to avoid real waits.
var (
	createKeyRetryDelay = 15 * time.Second
	createKeyMaxDelay   = 30 * time.Second

	// pendingCreationTimeout is the hard timeout for PENDING_CREATION keys.
	// After this duration without successful provisioning, the key transitions to ERROR.
	// It is a var (not const) so tests can override it.
	pendingCreationTimeout = 15 * time.Minute
)

var UnavailableKeyStates = []cmkapi.KeyState{
	cmkapi.KeyStatePENDINGDELETION,
	cmkapi.KeyStateDELETED,
	cmkapi.KeyStateFORBIDDEN,
	cmkapi.KeyStateUNKNOWN,
	cmkapi.KeyStatePENDINGCREATION,
	cmkapi.KeyStateERROR,
}

func IsUnavailableKeyState(state cmkapi.KeyState) bool {
	return slices.Contains(UnavailableKeyStates, state)
}

type KeyManager struct {
	ProviderConfigManager

	repo              repo.Repo
	keyConfigManager  *KeyConfigManager
	keyVersionManager *KeyVersionManager
	user              User
	eventFactory      *eventprocessor.EventFactory
	cmkAuditor        *auditor.Auditor
	asyncClient       async.Client
}

func NewKeyManager(
	repo repo.Repo,
	svcRegistry serviceapi.Registry,
	tenantConfigs *TenantConfigManager,
	keyConfigManager *KeyConfigManager,
	user User,
	certManager *CertificateManager,
	eventFactory *eventprocessor.EventFactory,
	cmkAuditor *auditor.Auditor,
	asyncClient async.Client,
) *KeyManager {
	keyVersionManager := NewKeyVersionManager(repo, svcRegistry, tenantConfigs, certManager, cmkAuditor)

	return &KeyManager{
		ProviderConfigManager: *NewProviderConfigManager(
			svcRegistry,
			make(map[ProviderCachedKey]*ProviderConfig),
			tenantConfigs,
			certManager,
			NewPool(repo),
			repo,
		),
		repo:              repo,
		keyConfigManager:  keyConfigManager,
		keyVersionManager: keyVersionManager,
		user:              user,
		eventFactory:      eventFactory,
		cmkAuditor:        cmkAuditor,
		asyncClient:       asyncClient,
	}
}

func (km *KeyManager) Create(
	ctx context.Context,
	key *model.Key,
) (*model.Key, error) {
	ctx = model.LogInjectKey(ctx, key)

	// Validate access and load key configuration
	if err := km.validateKeyCreation(ctx, key); err != nil {
		return nil, err
	}

	// For BYOK keys, check if tenant provisioning is needed before attempting provider creation.
	// If the default keystore has not yet had its management role provisioned (LocalityID == ""),
	// persist the key in PENDING_CREATION state and return immediately. The sync worker will
	// complete creation once provisioning succeeds.
	if key.KeyType == constants.KeyTypeBYOK {
		pending, err := km.createPendingBYOKKeyIfNeeded(ctx, key)
		if err != nil {
			return nil, err
		}
		if pending {
			return key, nil
		}
	}

	// Initialize provider
	provider, err := km.GetOrInitProvider(ctx, key)
	if err != nil {
		return nil, errs.Wrap(ErrFailedToInitProvider, err)
	}

	// Create or register key based on type
	keyResp, err := km.createOrRegisterProviderKey(ctx, key, provider)
	if err != nil {
		return nil, err
	}

	if err := km.persistCreatedKey(ctx, provider, key, keyResp); err != nil {
		return nil, err
	}

	km.sendCreateAuditLog(ctx, key)

	return key, nil
}

func (km *KeyManager) Get(ctx context.Context, keyID uuid.UUID) (*model.Key, error) {
	key := &model.Key{ID: keyID}

	joinCond := repo.JoinCondition{
		Table:     &model.Key{},
		Field:     repo.IDField,
		JoinTable: &model.KeyVersion{},
		JoinField: fmt.Sprintf("%s_%s", repo.KeyField, repo.IDField),
	}

	_, err := km.repo.First(
		ctx,
		key,
		*repo.NewQuery().
			Join(repo.LeftJoin, joinCond),
	)
	if err != nil {
		return nil, errs.Wrap(ErrGetKeyDB, err)
	}

	isPrimary, err := repo.IsPrimaryKey(ctx, km.repo, key)
	if err != nil {
		return nil, errs.Wrap(ErrGetKeyDB, err)
	}
	key.IsPrimary = isPrimary

	_, err = km.user.HasKeyAccess(ctx, authz.APIActionRead, key.KeyConfigurationID)
	if err != nil {
		return nil, err
	}

	switch key.KeyType {
	case cmkapi.KeyTypeBYOK:
	case cmkapi.KeyTypeHYOK:
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

	primaryKeyID, err := repo.GetKeyConfigPrimaryKey(ctx, km.repo, keyConfigID)
	if err != nil {
		return nil, 0, err
	}

	ck := repo.NewCompositeKey().Where(fmt.Sprintf("%s.%s", model.Key{}.TableName(), repo.KeyConfigIDField), keyConfigID)
	query = query.Where(repo.NewCompositeKeyGroup(ck))

	keys, count, err := repo.ListAndCount(ctx, km.repo, pagination, model.Key{}, query)
	if err != nil {
		return nil, 0, err
	}

	// All are non primary
	if primaryKeyID == nil {
		return keys, count, nil
	}

	for _, k := range keys {
		k.IsPrimary = *primaryKeyID == k.ID
	}

	return keys, count, nil
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

	if key.State == cmkapi.KeyStatePENDINGCREATION && keyPatch.Enabled != nil {
		return nil, ErrKeyInPendingState
	}

	err = km.handleCryptoDetailsUpdate(ctx, keyPatch, key)
	if err != nil {
		return nil, errs.Wrap(ErrCryptoDetailsUpdate, err)
	}

	if key.KeyType == cmkapi.KeyTypeHYOK && keyPatch.Enabled != nil {
		return nil, errs.Wrapf(ErrHYOKKeyActionNotAllowed, "update key state")
	}

	enablementUpdated := copyFieldsToModelKey(keyPatch, key)

	if err = km.applyKeyPatch(ctx, key, keyPatch, enablementUpdated); err != nil {
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
			repo.NewCompositeKey().Where(repo.KeyTypeField, cmkapi.KeyTypeHYOK),
		),
	)

	return repo.ProcessInBatch(ctx, km.repo, baseQuery, repo.DefaultLimit, func(keys []*model.Key) error {
		for _, key := range keys {
			err := km.syncHYOKKeyState(ctx, key)
			if err != nil {
				continue
			}
		}

		return nil
	})
}

// SyncPendingCreationKey processes a single BYOK key in PENDING_CREATION state.
// It attempts to complete provisioning and transitions the key to PENDING_IMPORT on success,
// or to ERROR on hard timeout.
func (km *KeyManager) SyncPendingCreationKey(ctx context.Context, keyID uuid.UUID) error {
	key, err := km.Get(ctx, keyID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			log.Debug(ctx, "PENDING_CREATION key no longer exists, skipping sync", slog.String("keyID", keyID.String()))
			return nil
		}
		return errs.Wrap(ErrGetKeyDB, err)
	}
	if key.State != cmkapi.KeyStatePENDINGCREATION {
		return nil
	}
	return km.syncPendingCreationKey(ctx, key)
}

func (km *KeyManager) Detach(ctx context.Context, key *model.Key) error {
	return km.repo.Transaction(ctx, func(ctx context.Context) error {
		key.State = cmkapi.KeyStateDETACHING

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

func (km *KeyManager) syncPendingCreationKey(ctx context.Context, key *model.Key) error {
	ctx = model.LogInjectKey(ctx, key)
	elapsed := time.Since(key.CreatedAt)
	elapsedDisplay := elapsed.Round(time.Second)
	ctx = slogctx.With(ctx, slog.Duration("elapsed", elapsedDisplay))

	// Check hard timeout: if the key has been in PENDING_CREATION for too long, transition to ERROR.
	if elapsed > pendingCreationTimeout {
		log.Error(ctx, "PENDING_CREATION key timed out, transitioning to ERROR", ErrProvisioningTimeout)
		return km.transitionPendingKeyToError(ctx, key,
			"PROVISIONING_TIMEOUT",
			"Key provisioning timed out. Delete this key and re-create it once the issue is resolved.")
	}

	log.Info(ctx, "Attempting to provision PENDING_CREATION key",
		slog.Duration("timeout", pendingCreationTimeout))

	provider, err := km.initProviderForPendingKey(ctx, key, elapsedDisplay)
	if err != nil {
		return err
	}
	if provider == nil {
		// Terminal non-retryable condition already handled (e.g. pool drained → ERROR state set).
		return nil
	}

	log.Info(ctx, "Provider initialised, creating key in keystore")

	if err := km.createOrRecoverProviderKey(ctx, key, provider, elapsedDisplay); err != nil {
		return err
	}

	// Persist the updated key (NativeID and State set by createManagedProviderKey)
	// and set primary if this is the first key for the configuration.
	err = km.repo.Transaction(ctx, func(ctx context.Context) error {
		if err := km.setPrimaryIfFirstKey(ctx, key); err != nil {
			return errs.Wrap(ErrUpdatePrimary, err)
		}
		_, err := km.repo.Patch(ctx, key, *repo.NewQuery().UpdateAll(true))
		if err != nil {
			return errs.Wrap(ErrUpdateKeyDB, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	log.Debug(ctx, "PENDING_CREATION key transitioned to PENDING_IMPORT",
		slog.String("newState", string(key.State)))

	return nil
}

// isBYOKProvisioningNeeded reports whether the default keystore's management role
// has not yet been provisioned for this tenant. It reads the stored config without
// triggering lazy provisioning, so it never blocks on GrantTrust.
func (km *KeyManager) isBYOKProvisioningNeeded(ctx context.Context) (bool, error) {
	needed, err := km.tenantConfigs.NeedsDefaultKeystoreProvisioning(ctx)
	if err != nil {
		return false, err
	}
	return needed, nil
}

func (km *KeyManager) persistCreatedKey(
	ctx context.Context,
	provider *ProviderConfig,
	key *model.Key,
	keyResp *keymanagement.GetKeyResponse,
) error {
	return km.repo.Transaction(ctx, func(ctx context.Context) error {
		if err := km.setPrimaryIfFirstKey(ctx, key); err != nil {
			return errs.Wrap(ErrUpdatePrimary, err)
		}
		if err := km.repo.Create(ctx, key); err != nil {
			return errs.Wrap(ErrCreateKeyDB, err)
		}
		if key.KeyType == constants.KeyTypeHYOK && keyResp != nil {
			if err := km.syncKeyVersions(ctx, provider, key); err != nil {
				return errs.Wrap(ErrCreateKeyVersionDB, err)
			}
		}
		return nil
	})
}

// createPendingBYOKKeyIfNeeded persists the key in PENDING_CREATION when tenant provisioning
// is not yet complete. Returns true if the key was saved as pending (caller should return early).
func (km *KeyManager) createPendingBYOKKeyIfNeeded(ctx context.Context, key *model.Key) (bool, error) {
	needsProvisioning, err := km.isBYOKProvisioningNeeded(ctx)
	if err != nil {
		return false, err
	}
	if !needsProvisioning {
		return false, nil
	}
	key.State = cmkapi.KeyStatePENDINGCREATION
	key.NativeID = nil // not yet created in provider
	if err := km.repo.Transaction(ctx, func(ctx context.Context) error {
		return km.repo.Create(ctx, key)
	}); err != nil {
		return false, errs.Wrap(ErrCreateKeyDB, err)
	}
	km.sendCreateAuditLog(ctx, key)
	km.enqueuePendingStateSync(ctx, key)
	return true, nil
}

func (km *KeyManager) applyKeyPatch(
	ctx context.Context,
	key *model.Key,
	keyPatch cmkapi.KeyPatch,
	enablementUpdated bool,
) error {
	return km.repo.Transaction(ctx, func(ctx context.Context) error {
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
}

// initProviderForPendingKey calls GetOrInitProvider and maps known error types to appropriate
// actions: ErrGrantTrustFailed returns the error for retry, ErrPoolIsDrained transitions to ERROR.
func (km *KeyManager) initProviderForPendingKey(
	ctx context.Context,
	key *model.Key,
	elapsed time.Duration,
) (*ProviderConfig, error) {
	ctx = slogctx.With(ctx, slog.Duration("elapsed", elapsed))
	provider, err := km.GetOrInitProvider(ctx, key)
	if err == nil {
		return provider, nil
	}
	if errors.Is(err, ErrGrantTrustFailed) {
		log.Info(ctx, "Provisioning still in progress: waiting for trust grant (WIF pool / IAM role)",
			log.ErrorAttr(err))
		return nil, err
	}
	if errors.Is(err, ErrPoolIsDrained) {
		log.Warn(ctx, "Provisioning failed: keystore pool is drained, transitioning to ERROR",
			log.ErrorAttr(err))
		return nil, km.transitionPendingKeyToError(ctx, key,
			"KEYSTORE_POOL_DRAINED",
			"No keystore available for key provisioning. "+
				"Delete this key and re-create it once the keystore pool is replenished.")
	}
	return nil, err
}

// createOrRecoverProviderKey creates the key in the provider. On AlreadyExists, attempts to
// recover the NativeID from a prior partial attempt instead of failing.
func (km *KeyManager) createOrRecoverProviderKey(
	ctx context.Context,
	key *model.Key,
	provider *ProviderConfig,
	elapsed time.Duration,
) error {
	ctx = slogctx.With(ctx, slog.Duration("elapsed", elapsed))
	err := km.createManagedProviderKey(ctx, key, provider)
	if err == nil {
		return nil
	}
	if status.Code(err) != codes.AlreadyExists {
		log.Info(ctx, "Provider key creation failed, will retry", log.ErrorAttr(err))
		return err
	}
	log.Info(ctx, "Provider key already exists (prior partial attempt), recovering NativeID")
	if recovErr := km.recoverExistingProviderKey(ctx, key, provider); recovErr != nil {
		log.Info(ctx, "Recovery of existing provider key failed, will retry", log.ErrorAttr(recovErr))
		return recovErr
	}
	return nil
}

// enqueuePendingStateSync immediately enqueues the pending state sync task so
// provisioning can begin right away. Errors are non-fatal and logged.
func (km *KeyManager) enqueuePendingStateSync(ctx context.Context, key *model.Key) {
	if km.asyncClient == nil {
		log.Warn(ctx, "async client not initialized, skipping pending state sync enqueue")
		return
	}
	payload := asyncUtils.NewTaskPayload(ctx, []byte(key.ID.String()))
	payloadBytes, err := payload.ToBytes()
	if err != nil {
		log.Error(ctx, "Failed to serialize pending state sync task payload", err)
		return
	}
	task := asynq.NewTask(config.TypePendingStateSync, payloadBytes)
	info, err := km.asyncClient.Enqueue(task)
	if err != nil {
		log.Error(ctx, "Failed to enqueue pending state sync task", err)
		return
	}
	log.Info(ctx, "Enqueued pending state sync task",
		slog.String("taskId", info.ID),
		slog.String("keyId", key.ID.String()))
}

func (km *KeyManager) transitionPendingKeyToError(ctx context.Context, key *model.Key, code, msg string) error {
	now := time.Now().UTC()
	detail := cmkapi.KeyErrorDetail{
		ErrorCode:      &code,
		ErrorMessage:   &msg,
		ErrorTimestamp: &now,
	}

	detailBytes, err := json.Marshal(detail)
	if err != nil {
		return err
	}

	key.State = cmkapi.KeyStateERROR
	key.ErrorDetail = detailBytes

	_, err = km.repo.Patch(ctx, key, *repo.NewQuery().UpdateAll(true))
	if err != nil {
		return errs.Wrap(ErrUpdateKeyDB, err)
	}

	return nil
}

// validateKeyCreation checks user access and loads key configuration
func (km *KeyManager) validateKeyCreation(ctx context.Context, key *model.Key) error {
	_, err := km.user.HasKeyAccess(ctx, authz.APIActionCreate, key.KeyConfigurationID)
	if err != nil {
		return err
	}

	keyConfig := &model.KeyConfiguration{ID: key.KeyConfigurationID}
	_, err = km.repo.First(ctx, keyConfig, *repo.NewQuery())
	if err != nil {
		return errs.Wrap(ErrGetConfiguration, err)
	}

	return nil
}

// createOrRegisterProviderKey creates a managed key or registers a HYOK key based on key type
func (km *KeyManager) createOrRegisterProviderKey(
	ctx context.Context,
	key *model.Key,
	provider *ProviderConfig,
) (*keymanagement.GetKeyResponse, error) {
	switch key.KeyType {
	case cmkapi.KeyTypeBYOK:
		return nil, km.createManagedProviderKey(ctx, key, provider)
	case cmkapi.KeyTypeHYOK:
		return km.registerHYOKKey(ctx, key, provider)
	default:
		return nil, ErrInvalidKeystore
	}
}

func (km *KeyManager) setEditableStatus(ctx context.Context, key *model.Key) error {
	cryptoData := key.GetCryptoAccessData()
	if cryptoData == nil {
		return nil
	}

	// By default HYOK keys can be editable
	for region := range cryptoData {
		key.EditableRegions[region] = key.KeyType == cmkapi.KeyTypeHYOK
	}

	// All regions for non primary keys are editable
	// Non-HYOK will not be editable, so we also end here
	if !key.IsPrimary || key.KeyType != cmkapi.KeyTypeHYOK {
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

//nolint:cyclop
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

	management, err := key.GetManagementAccessData()
	if err != nil {
		return err
	}
	keyPatch.AccessDetails.Management = &management

	err = providerTransformer.ValidateKeyAccessData(ctx, keyPatch.AccessDetails)
	if err != nil {
		return err
	}

	keyCryptoData := key.GetCryptoAccessData()
	for region, patchRegionValues := range *patchAccessDetails.Crypto {
		editable, exist := key.EditableRegions[region]
		if !editable && exist {
			// If region is not editable and content changed error
			if !maps.Equal(keyCryptoData[region].AdditionalProperties, patchRegionValues.AdditionalProperties) {
				return ErrNonEditableCryptoRegionUpdate
			}
		}

		if !exist {
			res, err := km.newCryptoRegion(ctx, region, patchRegionValues.AdditionalProperties)
			if err != nil {
				return err
			}
			keyCryptoData[region] = res
		} else {
			regionData := keyCryptoData[region]
			if regionData.AdditionalProperties == nil {
				regionData.AdditionalProperties = make(map[string]any)
			}
			maps.Copy(regionData.AdditionalProperties, patchRegionValues.AdditionalProperties)
			keyCryptoData[region] = regionData
		}
	}

	bytes, err := json.Marshal(keyCryptoData)
	if err != nil {
		return err
	}

	key.CryptoAccessData = bytes

	return nil
}

func (km *KeyManager) createManagedProviderKey(
	ctx context.Context,
	key *model.Key,
	provider *ProviderConfig,
) error {
	var keyResp *keymanagement.CreateKeyResponse

	err := retry.New(
		retry.RetryIf(func(err error) bool {
			return errors.Is(err, keymanagement.ErrProviderAuthenticationFailed)
		}),
		retry.Attempts(createKeyRetryAttempts),
		retry.Delay(createKeyRetryDelay),
		retry.MaxDelay(createKeyMaxDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		retry.Context(ctx),
	).Do(func() error {
		var err error
		keyID := key.ID.String()
		keyResp, err = provider.Client.CreateKey(ctx, &keymanagement.CreateKeyRequest{
			Config:       common.KeystoreConfig{Values: provider.Config.Values},
			KeyAlgorithm: convertToAPIKeyAlgorithm(key.Algorithm),
			ID:           &keyID,
			Region:       key.Region,
			KeyType:      convertToAPIKeyType(key.KeyType),
		})
		return err
	})
	if err != nil {
		return errs.Wrap(ErrKeyCreationFailed, err)
	}

	key.NativeID = &keyResp.KeyID
	key.State = cmkapi.KeyState(keyResp.Status)

	return nil
}

// recoverExistingProviderKey handles the case where CreateKey returned AlreadyExists,
// meaning a prior attempt created the key in the provider but never updated the DB.
// It calls GetKey to retrieve the existing key's NativeID and status.
func (km *KeyManager) recoverExistingProviderKey(
	ctx context.Context,
	key *model.Key,
	provider *ProviderConfig,
) error {
	keyResp, err := provider.Client.GetKey(ctx, &keymanagement.GetKeyRequest{
		Parameters: keymanagement.RequestParameters{
			Config: common.KeystoreConfig{Values: provider.Config.Values},
			KeyID:  key.ID.String(),
		},
	})
	if err != nil {
		return errs.Wrap(ErrKeyCreationFailed, err)
	}

	key.NativeID = &keyResp.KeyID
	key.State = cmkapi.KeyState(keyResp.Status)

	return nil
}

// registerHYOKKey validates that the HYOK key exists in the customer's keystore
// and returns the key information for version creation after the key is saved.
func (km *KeyManager) registerHYOKKey(
	ctx context.Context,
	key *model.Key,
	provider *ProviderConfig,
) (*keymanagement.GetKeyResponse, error) {
	configValues, err := mergeProviderConfigValuesWithKeyAccessData(provider, key)
	if err != nil {
		return nil, err
	}

	keyResp, err := provider.Client.GetKey(ctx, &keymanagement.GetKeyRequest{
		Parameters: keymanagement.RequestParameters{
			Config: common.KeystoreConfig{Values: configValues},
			KeyID:  *key.NativeID,
		},
	})
	if err != nil {
		return nil, errs.Wrap(ErrKeyRegistration, err)
	}

	err = km.addCertificateSubjectToCryptoData(ctx, key)
	if err != nil {
		return nil, errs.Wrap(ErrKeyRegistration, err)
	}

	if keyResp.KeyAlgorithm != keymanagement.AES256 {
		return nil, errs.Wrapf(
			ErrUnsupportedKeyAlgorithm,
			fmt.Sprintf("%v for HYOK registration", keyResp.KeyAlgorithm),
		)
	}

	key.Algorithm = cmkapi.KeyAlgorithmAES256

	if cmkapi.KeyState(keyResp.Status) != cmkapi.KeyStateENABLED {
		return nil, errs.Wrapf(
			ErrInvalidKeyState,
			keyResp.Status+" for HYOK registration",
		)
	}

	key.State = cmkapi.KeyStateENABLED

	// Initial KeyVersion will be created after key is saved via syncKeyVersion
	// This ensures proper use of RotationTime from keystore and consistent version creation logic

	log.Debug(
		ctx,
		"Key Register",
		slog.Group(
			"Provider Key",
			slog.String("id", keyResp.KeyID),
			slog.String("status", keyResp.Status),
			slog.String("version", keyResp.LatestKeyVersionId),
		),
	)

	return keyResp, nil
}

func (km *KeyManager) addCertificateSubjectToCryptoData(ctx context.Context, key *model.Key) error {
	cryptoCerts, err := km.certs.getCryptoCertificates(ctx)
	if err != nil {
		return err
	}

	cryptoAccessData := key.GetCryptoAccessData()
	if cryptoAccessData == nil {
		return nil
	}

	for _, cert := range cryptoCerts {
		accessData, exists := cryptoAccessData[cert.Name]
		if !exists {
			continue
		}

		subject := cert.Subject.String()
		accessData.CertificateSubject = &subject
		cryptoAccessData[cert.Name] = accessData
	}

	key.CryptoAccessData, err = json.Marshal(cryptoAccessData)
	if err != nil {
		return err
	}

	return nil
}

func (km *KeyManager) newCryptoRegion(
	ctx context.Context,
	region string,
	properties map[string]any,
) (cmkapi.KeyAccessDetailsRegion, error) {
	cryptoCerts, err := km.certs.getCryptoCertificates(ctx)
	if err != nil {
		return cmkapi.KeyAccessDetailsRegion{}, err
	}

	var certName string
	for _, cert := range cryptoCerts {
		if cert.Name == region {
			certName = region
			break
		}
	}

	return cmkapi.KeyAccessDetailsRegion{
		CertificateSubject:   &certName,
		AdditionalProperties: properties,
	}, nil
}

func (km *KeyManager) deleteProviderKey(ctx context.Context, key *model.Key) error {
	// If the key is a HYOK key, we do not delete it from the provider
	if key.KeyType == cmkapi.KeyTypeHYOK {
		return nil
	}

	// PENDING_CREATION keys have no NativeID yet — nothing to delete from the provider.
	if key.State == cmkapi.KeyStatePENDINGCREATION {
		return nil
	}

	provider, err := km.GetOrInitProvider(ctx, key)
	if err != nil {
		return errs.Wrap(ErrFailedToInitProvider, err)
	}

	switch key.KeyType {
	case constants.KeyTypeSystemManaged:
		return km.deleteSystemManagedProviderKey(ctx, key, provider)
	case constants.KeyTypeBYOK:
		// For BYOK keys, we delete the key itself, since BYOK keys are not versioned.
		// NativeID may be nil if the key never completed provisioning.
		if key.NativeID == nil {
			return nil
		}
		_, err = provider.Client.DeleteKey(ctx, &keymanagement.DeleteKeyRequest{
			Parameters: keymanagement.RequestParameters{
				Config: common.KeystoreConfig{Values: maps.Clone(provider.Config.Values)},
				KeyID:  *key.NativeID,
			},
		})
		if err != nil {
			return errs.Wrap(ErrFailedToDeleteProvider, err)
		}
	case cmkapi.KeyTypeHYOK:
		// HYOK keys are managed externally; nothing to delete on the provider side.
	}

	return nil
}

func (km *KeyManager) deleteSystemManagedProviderKey(
	ctx context.Context,
	key *model.Key,
	provider *ProviderConfig,
) error {
	for _, kv := range key.KeyVersions {
		_, err := provider.Client.DeleteKey(ctx, &keymanagement.DeleteKeyRequest{
			Parameters: keymanagement.RequestParameters{
				Config: common.KeystoreConfig{Values: maps.Clone(provider.Config.Values)},
				KeyID:  kv.NativeID,
			},
		})
		if err != nil {
			return errs.Wrap(ErrFailedToDeleteProvider, err)
		}
	}
	return nil
}

func (km *KeyManager) reenableProviderKey(ctx context.Context, key *model.Key) error {
	provider, err := km.GetOrInitProvider(ctx, key)
	if err != nil {
		return errs.Wrap(ErrFailedToInitProvider, err)
	}

	wasProviderError := false

	_, err = provider.Client.EnableKey(ctx, &keymanagement.EnableKeyRequest{
		Parameters: keymanagement.RequestParameters{
			Config: common.KeystoreConfig{Values: maps.Clone(provider.Config.Values)},
			KeyID:  *key.NativeID,
		},
	})
	if err != nil {
		wasProviderError = true
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
		if key.State == cmkapi.KeyStatePENDINGCREATION {
			return nil // Not primary yet; skip until creation completes
		}
		if key.State == cmkapi.KeyStateDISABLED {
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

func (km *KeyManager) disableProviderKey(ctx context.Context, key *model.Key) error {
	provider, err := km.GetOrInitProvider(ctx, key)
	if err != nil {
		return errs.Wrap(ErrFailedToInitProvider, err)
	}

	wasProviderError := false

	_, err = provider.Client.DisableKey(ctx, &keymanagement.DisableKeyRequest{
		Parameters: keymanagement.RequestParameters{
			Config: common.KeystoreConfig{Values: maps.Clone(provider.Config.Values)},
			KeyID:  *key.NativeID,
		},
	})
	if err != nil {
		wasProviderError = true
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
		if *apiKey.Enabled && dbKey.State != cmkapi.KeyStateENABLED {
			dbKey.State = cmkapi.KeyStateENABLED
			enablementUpdated = true
		} else if !(*apiKey.Enabled) && dbKey.State != cmkapi.KeyStateDISABLED {
			dbKey.State = cmkapi.KeyStateDISABLED
			enablementUpdated = true
		}
	}

	return enablementUpdated
}

func mergeProviderConfigValuesWithKeyAccessData(
	provider *ProviderConfig,
	key *model.Key,
) (map[string]any, error) {
	// Start with the provider config values
	configValues := provider.Config.Values

	management, err := key.GetManagementAccessData()
	if err != nil {
		return nil, err
	}

	// Create a copy to avoid modifying the original
	merged := make(map[string]any, len(configValues)+len(management.AdditionalProperties))
	maps.Copy(merged, configValues)

	// At this point, we assume the access data is already validated
	// in the API layer, so we can directly merge it.
	maps.Copy(merged, management.AdditionalProperties)

	return merged, nil
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

	var authzAction authz.APIAction
	switch action {
	case BYOKActionGetImportParams:
		authzAction = authz.APIActionRead
		if key.KeyType != cmkapi.KeyTypeBYOK {
			return nil, errs.Wrapf(ErrInvalidKeyTypeForImportParams,
				fmt.Sprintf("key type %s is not supported", key.KeyType))
		}
		if key.State != cmkapi.KeyStatePENDINGIMPORT {
			return nil, errs.Wrapf(ErrInvalidKeyStateForImportParams,
				fmt.Sprintf("key state %s is not supported", key.State))
		}
	case BYOKActionImportKeyMaterial:
		authzAction = authz.APIActionUpdate
		if key.KeyType != cmkapi.KeyTypeBYOK {
			return nil, errs.Wrapf(ErrInvalidKeyTypeForImportKeyMaterial,
				fmt.Sprintf("key type %s is not supported", key.KeyType))
		}
		if key.State != cmkapi.KeyStatePENDINGIMPORT {
			return nil, errs.Wrapf(ErrInvalidKeyStateForImportKeyMaterial,
				fmt.Sprintf("key state %s is not supported", key.State))
		}
	default:
		return nil, ErrInvalidBYOKAction
	}

	_, err = km.user.HasKeyAccess(ctx, authzAction, key.KeyConfigurationID)
	if err != nil {
		return nil, err
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
		err = km.repo.Set(ctx, importParams, *repo.NewQuery())
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

	key.State = cmkapi.KeyState(keyResp.Status)

	return key, nil
}

// handleSystemsOnKeyRotation sends SYSTEM_KEY_ROTATE events to all systems
// connected to the key's KeyConfiguration. This is triggered when a primary key's
// key material is rotated (new version detected).
func (km *KeyManager) handleSystemsOnKeyRotation(ctx context.Context, key *model.Key) error {
	log.Info(ctx, "notifying systems of key rotation",
		slog.String("keyID", key.ID.String()),
		slog.String("keyConfigID", key.KeyConfigurationID.String()))

	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(
				repo.KeyConfigIDField, key.KeyConfigurationID,
			),
		),
	)

	return repo.ProcessInBatch(
		ctx,
		km.repo,
		query,
		repo.DefaultLimit,
		func(systems []*model.System) error {
			for _, s := range systems {
				log.Debug(ctx, "sending rotation event to system",
					slog.String("systemID", s.ID.String()),
					slog.String("keyID", key.ID.String()))

				_, err := km.eventFactory.SystemKeyRotate(
					ctx,
					s,
					key.ID.String(),
				)
				if err != nil {
					log.Error(ctx, "failed to create rotation event", err,
						slog.String("systemID", s.ID.String()),
						slog.String("keyID", key.ID.String()))
					return err
				}
			}

			return nil
		},
	)
}

func (km *KeyManager) syncHYOKKeyState(ctx context.Context, key *model.Key) error {
	ctx = model.LogInjectKey(ctx, key)
	oldKeyState := key.State

	if key.KeyType != cmkapi.KeyTypeHYOK {
		return ErrInvalidKeyTypeForHYOKSync
	}

	provider, err := km.GetOrInitProvider(ctx, key)
	if err != nil {
		err = errs.Wrap(ErrFailedToInitProvider, err)
		log.Error(ctx, "Failed to sync HYOK key state with provider", err, slog.String("keyID", key.ID.String()))
	}

	keyResp, err := km.getKeyFromProvider(ctx, provider, key)
	if err != nil {
		log.Error(ctx, "Failed to sync HYOK key state with provider", err, slog.String("keyID", key.ID.String()))
		key.State = km.getKeyStateOnSyncError(ctx, key, err)
	} else if keyResp != nil {
		// Successful case - update the status in the database for the HYOK key Enabled/Disabled
		key.State = cmkapi.KeyState(keyResp.Status)

		// Check if a new version was detected from the keystore
		err := km.syncKeyVersions(ctx, provider, key)
		if err != nil {
			log.Warn(ctx, "Failed to sync key version", log.ErrorAttr(err))
		}
	}

	if oldKeyState == key.State {
		return nil
	}

	err = km.repo.Transaction(ctx, func(ctx context.Context) error {
		_, err := km.repo.Patch(ctx, key, *repo.NewQuery())
		if err != nil {
			return err
		}

		return km.handleKeyStateTransition(ctx, key, oldKeyState)
	})
	if err != nil {
		return errs.Wrap(ErrUpdateKeyDB, err)
	}

	return nil
}

// syncKeyVersions checks if the latest version from keystore matches the stored version.
// If a new version is detected, it creates a new KeyVersion record.
func (km *KeyManager) syncKeyVersions(
	ctx context.Context,
	provider *ProviderConfig,
	key *model.Key,
) error {
	keyResp, err := km.getKeyVersionsFromProvider(ctx, provider, key)
	if err != nil {
		return err
	}

	if len(keyResp.Versions) < 1 {
		return ErrNoKeyVersionsFound
	}

	return km.handleNewKeyVersion(ctx, key, keyResp)
}

func (km *KeyManager) handleNewKeyVersion(
	ctx context.Context,
	key *model.Key,
	keyResp *keymanagement.GetKeyVersionsResponse,
) error {
	// New version detected - create it
	err := km.keyVersionManager.UpdateVersions(
		ctx,
		key.ID,
		keyResp.Versions,
	)
	if err != nil {
		return err
	}

	log.Debug(
		ctx, "Created new key version",
		slog.String("keyId", key.ID.String()),
	)

	// Send audit log for rotation detection
	km.sendRotateAuditLog(ctx, key)

	isPrimary, err := repo.IsPrimaryKey(ctx, km.repo, key)
	if err != nil {
		return err
	}

	// Notify systems if this is a primary key
	if isPrimary {
		if err := km.handleSystemsOnKeyRotation(ctx, key); err != nil {
			// Log error but don't fail the version creation
			// Systems will get updated on next scheduled sync
			log.Error(ctx, "failed to notify systems of key rotation", err,
				slog.String("keyID", key.ID.String()))
		}
	}

	return nil
}

func (km *KeyManager) handleKeyStateTransition(ctx context.Context, key *model.Key, oldKeyState cmkapi.KeyState) error {
	switch key.State {
	case cmkapi.KeyStateENABLED:
		if !IsUnavailableKeyState(oldKeyState) {
			km.sendEnableAuditLog(ctx, key)
		}

		return km.sendEnableEvent(ctx, key)
	case cmkapi.KeyStatePENDINGDELETION:
		return nil
	case cmkapi.KeyStateDISABLED:
		if !IsUnavailableKeyState(oldKeyState) {
			km.sendDisableAuditLog(ctx, key)
		}

		return km.sendDisableEvent(ctx, key)
	default:
		return nil
	}
}

func (km *KeyManager) getKeyFromProvider(
	ctx context.Context,
	provider *ProviderConfig,
	key *model.Key,
) (*keymanagement.GetKeyResponse, error) {
	if provider == nil {
		return nil, ErrFailedToInitProvider
	}

	configValues, err := mergeProviderConfigValuesWithKeyAccessData(provider, key)
	if err != nil {
		return nil, err
	}

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

func (km *KeyManager) getKeyVersionsFromProvider(
	ctx context.Context,
	provider *ProviderConfig,
	key *model.Key,
) (*keymanagement.GetKeyVersionsResponse, error) {
	configValues, err := mergeProviderConfigValuesWithKeyAccessData(provider, key)
	if err != nil {
		return nil, err
	}

	keyResp, err := provider.Client.GetKeyVersions(ctx, &keymanagement.GetKeyVersionsRequest{
		Parameters: keymanagement.RequestParameters{
			Config: common.KeystoreConfig{Values: configValues},
			KeyID:  *key.NativeID,
		},
	})
	if err != nil {
		return nil, errs.Wrap(ErrGetProviderKeyVersions, err)
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

func (km *KeyManager) getKeyStateOnSyncError(ctx context.Context, key *model.Key, err error) cmkapi.KeyState {
	var newState cmkapi.KeyState

	switch {
	case errors.Is(err, keymanagement.ErrProviderAuthenticationFailed):
		newState = cmkapi.KeyStateFORBIDDEN
	case errors.Is(err, keymanagement.ErrHYOKKeyNotFound):
		newState = cmkapi.KeyStateDELETED
	case errs.IsAnyError(err, ErrFailedToInitProvider, ErrGetProviderKey):
		newState = cmkapi.KeyStateUNKNOWN
	default:
		log.Warn(
			ctx, "Failed to sync HYOK key due to unhandled error, keeping existing state",
			log.ErrorAttr(err),
		)

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

func (km *KeyManager) sendRotateAuditLog(ctx context.Context, key *model.Key) {
	err := km.cmkAuditor.SendCmkRotateAuditLog(ctx, key.ID.String())
	if err != nil {
		log.Error(ctx, "Failed to send audit log for CMK Rotate", err)
		return
	}

	log.Info(ctx, "Audit log for CMK Rotate sent successfully")
}

func (km *KeyManager) enableKey(ctx context.Context, key *model.Key) error {
	err := km.reenableProviderKey(ctx, key)
	if err != nil {
		return err
	}

	km.sendEnableAuditLog(ctx, key)

	return km.sendEnableEvent(ctx, key)
}

func (km *KeyManager) disableKey(ctx context.Context, key *model.Key) error {
	err := km.disableProviderKey(ctx, key)
	if err != nil {
		return err
	}

	km.sendDisableAuditLog(ctx, key)

	return km.sendDisableEvent(ctx, key)
}

func convertToAPIKeyAlgorithm(alg cmkapi.KeyAlgorithm) keymanagement.KeyAlgorithm {
	if alg == cmkapi.KeyAlgorithmAES256 {
		return keymanagement.AES256
	}

	return keymanagement.UnspecifiedKeyAlgorithm
}

func convertToAPIKeyType(keyType cmkapi.KeyType) keymanagement.KeyType {
	switch keyType {
	case cmkapi.KeyTypeBYOK:
		return keymanagement.BYOK
	case cmkapi.KeyTypeHYOK:
		return keymanagement.HYOK
	default:
		return keymanagement.UnspecifiedKeyType
	}
}
