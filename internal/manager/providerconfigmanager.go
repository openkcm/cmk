package manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/google/uuid"

	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/common"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keystoremanagement"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	pluginHelpers "github.com/openkcm/cmk/utils/plugins"
)

const (
	DefaultProviderConfigCacheExpiration = 24 * time.Hour
)

var (
	ErrCreateKeystore                = errors.New("failed to create keystore")
	ErrInvalidKeystore               = errors.New("invalid keystore")
	ErrGetTenantFromCtx              = errors.New("failed to get tenant from context")
	ErrGetDefaultTenantCertificate   = errors.New("failed to get default tenant HYOK certificate")
	ErrGetDefaultKeystoreCertificate = errors.New("failed to get default keystore certificate")
	ErrAddConfigToPool               = errors.New("failed to add keystore configuration to pool")
	ErrCountKeystorePool             = errors.New("failed to get keystore pool size")
	ErrListPendingKeystores          = errors.New("failed to list pending keystores")
	ErrGrantTrustFailed              = errors.New("failed to grant trust to certificate")
)

type ProviderConfig struct {
	Config     *common.KeystoreConfig
	Client     keymanagement.KeyManagement
	Expiration time.Time // Optional expiration time for the provider config
}

func NewProviderConfig(
	config *common.KeystoreConfig,
	client keymanagement.KeyManagement,
	expiration *time.Time,
) *ProviderConfig {
	if expiration == nil {
		expiration = new(time.Now().Add(DefaultProviderConfigCacheExpiration)) // Default expiration if nil
	}

	return &ProviderConfig{
		Config:     config,
		Client:     client,
		Expiration: *expiration,
	}
}

func (c ProviderConfig) IsExpired() bool {
	return c.Expiration.Before(time.Now())
}

type ProviderConfigManager struct {
	svcRegistry   serviceapi.Registry
	providers     map[ProviderCachedKey]*ProviderConfig
	mu            sync.RWMutex
	tenantConfigs *TenantConfigManager
	certs         *CertificateManager
	repo          repo.Repo
	keystorePool  *Pool
}

func NewProviderConfigManager(
	svcRegistry serviceapi.Registry,
	providers map[ProviderCachedKey]*ProviderConfig,
	tenantConfigs *TenantConfigManager,
	certs *CertificateManager,
	pool *Pool,
	repo repo.Repo,
) *ProviderConfigManager {
	return &ProviderConfigManager{
		svcRegistry:   svcRegistry,
		providers:     providers,
		mu:            sync.RWMutex{},
		tenantConfigs: tenantConfigs,
		certs:         certs,
		repo:          repo,
		keystorePool:  pool,
	}
}

const (
	pluginAlgorithmPrefix = "KEY_ALGORITHM_"
)

// getPluginAlgorithm returns the plugin algorithm for the key
func getPluginAlgorithm(alg string) string {
	return pluginAlgorithmPrefix + alg
}

type ProviderCachedKey struct {
	KeyStore string
	Provider string
	Tenant   string
}

func (k ProviderCachedKey) String() string {
	return k.KeyStore + ":" + k.Provider + ":" + k.Tenant
}

//nolint:funlen,cyclop
func (pmc *ProviderConfigManager) GetOrInitProvider(ctx context.Context, key *model.Key) (*ProviderConfig, error) {
	tenant, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return nil, errs.Wrap(ErrGetTenantFromCtx, err)
	}

	keystoreName := constants.DefaultKeyStore
	if key.KeyType == cmkapi.KeyTypeHYOK {
		keystoreName = constants.HYOKKeyStore
	}

	provider := key.Provider
	if keystoreName == constants.DefaultKeyStore {
		provider, err = pmc.GetDefaultKeystoreFromCatalog()
		if err != nil {
			return nil, err
		}
	}

	compositeKey := ProviderCachedKey{
		KeyStore: keystoreName,
		Provider: provider,
		Tenant:   tenant,
	}

	// First try read-only access
	pmc.mu.RLock()
	cfg, exists := pmc.providers[compositeKey]
	pmc.mu.RUnlock()

	if exists && !cfg.IsExpired() {
		return cfg, nil
	}

	// Need to initialize - acquire write lock
	pmc.mu.Lock()
	defer pmc.mu.Unlock()

	// Double-check after acquiring write lock
	if cfg, exists := pmc.providers[compositeKey]; exists && !cfg.IsExpired() {
		return cfg, nil
	}

	// Initialize config
	log.Debug(ctx, "Initializing Provider",
		slog.String("keystore", keystoreName),
		slog.String("provider", provider),
	)

	config, expiration, err := pmc.getKeystoreConfig(ctx, keystoreName)
	if err != nil {
		return nil, errs.Wrap(ErrConfigNotFound, err)
	}

	// Initialize client
	keyManagements, err := pmc.svcRegistry.KeyManagements()
	if err != nil {
		return nil, errs.Wrapf(ErrPluginNotFound, provider)
	}

	client, ok := keyManagements[provider]
	if !ok {
		return nil, errs.Wrapf(ErrPluginNotFound, provider)
	}

	providerCfg := NewProviderConfig(config, client, expiration)

	pmc.providers[compositeKey] = providerCfg

	return providerCfg, nil
}

// pendingKeystoreConfig is the JSON stored while a keystore waits for activation.
type pendingKeystoreConfig struct {
	AccountID string         `json:"accountId"`
	Values    map[string]any `json:"values"`
}

func (pmc *ProviderConfigManager) FillKeystorePool(ctx context.Context, size int) error {
	if err := pmc.reconcilePendingKeystores(ctx); err != nil {
		return err
	}

	activeCount, err := pmc.keystorePool.Count(ctx)
	if err != nil {
		return errs.Wrap(ErrCountKeystorePool, err)
	}

	pendingCount, err := pmc.keystorePool.CountPending(ctx)
	if err != nil {
		return errs.Wrap(ErrCountKeystorePool, err)
	}

	totalCount := activeCount + pendingCount

	log.Debug(ctx, "Filling keystore pool",
		slog.Int("activeCount", activeCount),
		slog.Int("pendingCount", pendingCount),
		slog.Int("totalCount", totalCount),
		slog.Int("targetSize", size),
	)

	for i := totalCount; i < size; i++ {
		req := &keystoremanagement.CreateKeystoreRequest{Values: map[string]any{}}
		provider, resp, err := pmc.CreateKeystore(ctx, req)
		if err != nil {
			return err
		}

		err = pmc.persistCreatedKeystore(ctx, provider, resp, req.Values)
		if err != nil {
			return err
		}
	}

	log.Debug(ctx, "Keystore pool fill finished",
		slog.Int("targetSize", size),
	)

	return nil
}

func (pmc *ProviderConfigManager) reconcilePendingKeystores(ctx context.Context) error {
	pending, err := pmc.keystorePool.GetPending(ctx)
	if err != nil {
		return errs.Wrap(ErrListPendingKeystores, err)
	}

	for _, ks := range pending {
		err := pmc.reconcilePendingKeystore(ctx, ks)
		if err != nil {
			log.Error(ctx, "Skipping pending keystore reconciliation", err,
				slog.String("keystoreID", ks.ID.String()),
			)
		}
	}

	return nil
}

func (pmc *ProviderConfigManager) reconcilePendingKeystore(ctx context.Context, ks *model.Keystore) error {
	var record pendingKeystoreConfig
	if err := json.Unmarshal(ks.Config, &record); err != nil || record.AccountID == "" {
		log.Error(ctx, "Pending keystore is missing account id", ErrInvalidKeystore,
			slog.String("keystoreID", ks.ID.String()),
		)
		ks.Status = model.KeystoreStatusFailed

		return pmc.keystorePool.Update(ctx, ks)
	}

	client, err := pmc.keystoreManagementClient(ks.Provider)
	if err != nil {
		return err
	}

	statusResp, err := client.GetKeystoreStatus(ctx, &keystoremanagement.GetKeystoreStatusRequest{
		AccountID: record.AccountID,
	})
	if err != nil {
		return err
	}

	switch statusResp.Status {
	case keystoremanagement.CreationStatusPendingActivation, "":
		return nil
	case keystoremanagement.CreationStatusFailed:
		ks.Status = model.KeystoreStatusFailed
		return pmc.keystorePool.Update(ctx, ks)
	case keystoremanagement.CreationStatusActive:
		return pmc.finalizePendingKeystore(ctx, ks, client, record)
	default:
		return nil
	}
}

func (pmc *ProviderConfigManager) finalizePendingKeystore(
	ctx context.Context,
	ks *model.Keystore,
	client keystoremanagement.KeystoreManagement,
	record pendingKeystoreConfig,
) error {
	values := record.Values
	if values == nil {
		values = map[string]any{}
	}

	resp, err := client.FinalizeKeystoreSetup(ctx, &keystoremanagement.FinalizeKeystoreSetupRequest{
		AccountID: record.AccountID,
		Values:    values,
	})
	if err != nil {
		return err
	}

	if resp.Status == keystoremanagement.CreationStatusFailed ||
		(resp.Status == "" && resp.ErrorMessage != "") {
		log.Error(ctx, "Finalizing keystore setup failed", ErrCreateKeystore,
			slog.String("keystoreID", ks.ID.String()),
			slog.String("accountID", record.AccountID),
			slog.String("errorMessage", resp.ErrorMessage),
		)
		ks.Status = model.KeystoreStatusFailed

		return pmc.keystorePool.Update(ctx, ks)
	}

	if resp.Status != "" && resp.Status != keystoremanagement.CreationStatusActive {
		return nil
	}

	raw, err := json.Marshal(resp.ToKeystoreConfig().Values)
	if err != nil {
		return errs.Wrap(ErrMarshalConfig, err)
	}

	ks.Config = raw
	ks.Status = model.KeystoreStatusActive

	return pmc.keystorePool.Update(ctx, ks)
}

func (pmc *ProviderConfigManager) CreateKeystore(
	ctx context.Context,
	req *keystoremanagement.CreateKeystoreRequest,
) (string, *keystoremanagement.CreateKeystoreResponse, error) {
	if req == nil {
		req = &keystoremanagement.CreateKeystoreRequest{}
	}
	if req.Values == nil {
		req.Values = map[string]any{}
	}

	provider, err := pmc.GetDefaultKeystoreFromCatalog()
	if err != nil {
		return "", nil, err
	}

	client, err := pmc.keystoreManagementClient(provider)
	if err != nil {
		return "", nil, err
	}

	resp, err := client.CreateKeystore(ctx, req)
	if err != nil {
		return "", nil, errs.Wrapf(ErrCreateKeystore, fmt.Sprintf("provider: %s, error: %v", provider, err))
	}

	// Plugins that omit status are treated as immediately active.
	if resp.Status == "" {
		resp.Status = keystoremanagement.CreationStatusActive
	}

	return provider, resp, nil
}

func (pmc *ProviderConfigManager) persistCreatedKeystore(
	ctx context.Context,
	provider string,
	resp *keystoremanagement.CreateKeystoreResponse,
	values map[string]any,
) error {
	switch resp.Status {
	case keystoremanagement.CreationStatusActive:
		return pmc.AddKeystoreToPool(ctx, provider, model.KeystoreStatusActive, resp.ToKeystoreConfig().Values)
	case keystoremanagement.CreationStatusPendingActivation:
		if resp.AccountID == "" {
			log.Error(ctx, "Skipping pending keystore without account id", ErrCreateKeystore,
				slog.String("provider", provider),
			)

			return nil
		}

		return pmc.AddKeystoreToPool(
			ctx,
			provider,
			model.KeystoreStatusPendingActivation,
			newPendingKeystoreConfig(resp.AccountID, values).asMap(),
		)
	case keystoremanagement.CreationStatusFailed:
		log.Error(ctx, "Keystore creation failed", ErrCreateKeystore,
			slog.String("provider", provider),
			slog.String("accountID", resp.AccountID),
			slog.String("errorMessage", resp.ErrorMessage),
		)
		if resp.AccountID == "" {
			return nil
		}

		return pmc.AddKeystoreToPool(
			ctx,
			provider,
			model.KeystoreStatusFailed,
			newPendingKeystoreConfig(resp.AccountID, values).asMap(),
		)
	default:
		log.Error(ctx, "Skipping keystore with unknown creation status", ErrCreateKeystore,
			slog.String("provider", provider),
			slog.String("status", string(resp.Status)),
		)

		return nil
	}
}

func newPendingKeystoreConfig(accountID string, values map[string]any) pendingKeystoreConfig {
	if values == nil {
		values = map[string]any{}
	}

	return pendingKeystoreConfig{
		AccountID: accountID,
		Values:    values,
	}
}

func (c pendingKeystoreConfig) asMap() map[string]any {
	return map[string]any{
		"accountId": c.AccountID,
		"values":    c.Values,
	}
}

func (pmc *ProviderConfigManager) keystoreManagementClient(
	provider string,
) (keystoremanagement.KeystoreManagement, error) {
	keystoreManagements, err := pmc.svcRegistry.KeystoreManagements()
	if err != nil {
		return nil, errs.Wrapf(ErrPluginNotFound, provider)
	}

	client, ok := keystoreManagements[provider]
	if !ok {
		return nil, errs.Wrapf(ErrPluginNotFound, provider)
	}

	return client, nil
}

func (pmc *ProviderConfigManager) AddKeystoreToPool(
	ctx context.Context,
	provider string,
	status string,
	config map[string]any,
) error {
	if config == nil {
		config = map[string]any{}
	}

	ksConfig, err := json.Marshal(config)
	if err != nil {
		return errs.Wrap(ErrMarshalConfig, err)
	}

	_, err = pmc.keystorePool.Add(ctx, &model.Keystore{
		ID:       uuid.New(),
		Provider: provider,
		Config:   ksConfig,
		Status:   status,
	})
	if err != nil {
		return errs.Wrap(ErrAddConfigToPool, err)
	}

	return nil
}

func (pmc *ProviderConfigManager) GetDefaultKeystoreFromCatalog() (string, error) {
	if pmc.svcRegistry == nil {
		return "", errs.Wrapf(ErrGetDefaultKeystore, "no plugin catalog available")
	}

	plugins, err := pmc.svcRegistry.KeyManagementList()
	if err != nil || len(plugins) == 0 {
		return "", errs.Wrapf(ErrGetDefaultKeystore, "no keystore plugins found in catalog")
	}

	providers := make([]string, 0)

	for _, plugin := range plugins {
		if pluginHelpers.HasTag(plugin.ServiceInfo().Tags(), constants.DefaultKeyStore) {
			providers = append(providers, plugin.ServiceInfo().Name())
		}
	}

	if len(providers) == 0 {
		return "", errs.Wrapf(ErrGetDefaultKeystore, "no keystore provider selected as default")
	}

	if len(providers) > 1 {
		return "", errs.Wrapf(ErrGetDefaultKeystore,
			fmt.Sprintf("multiple keystore providers found as default: %v", providers))
	}

	return providers[0], nil
}

func (pmc *ProviderConfigManager) getKeystoreConfig(
	ctx context.Context,
	keystoreName string,
) (*common.KeystoreConfig, *time.Time, error) {
	switch keystoreName {
	case constants.DefaultKeyStore:
		return pmc.getDefaultKeystoreConfig(ctx)
	case constants.HYOKKeyStore:
		return pmc.getHYOKKeystoreConfig(ctx)
	default:
		return nil, nil, ErrInvalidKeystore
	}
}

func (pmc *ProviderConfigManager) getDefaultKeystoreConfig(
	ctx context.Context,
) (*common.KeystoreConfig, *time.Time, error) {
	ksConfig, err := pmc.tenantConfigs.GetDefaultKeystoreConfig(ctx)
	if err != nil {
		return nil, nil, err
	}

	keyManagementCert, err := pmc.certs.getDefaultKeystoreClientCert(
		ctx,
		ksConfig.KeyManagementConfig.LocalityID,
		ksConfig.KeyManagementConfig.CommonName,
		model.CertificatePurposeKeyManagement,
	)
	if err != nil {
		return nil, nil, err
	}

	configMap := map[string]any{
		"authType":   constants.AuthTypeCertificate,
		"clientCert": keyManagementCert.CertPEM,
		"privateKey": keyManagementCert.PrivateKeyPEM,
	}

	maps.Copy(configMap, ksConfig.KeyManagementConfig.AccessData)

	return &common.KeystoreConfig{Values: configMap}, &keyManagementCert.ExpirationDate, nil
}

func (pmc *ProviderConfigManager) getHYOKKeystoreConfig(
	ctx context.Context,
) (*common.KeystoreConfig, *time.Time, error) {
	cert, err := pmc.certs.getDefaultHYOKClientCert(ctx)
	if err != nil {
		return nil, nil, err
	}

	configMap := map[string]any{
		"authType":   constants.AuthTypeCertificate,
		"clientCert": cert.CertPEM,
		"privateKey": cert.PrivateKeyPEM,
	}

	return &common.KeystoreConfig{Values: configMap}, &cert.ExpirationDate, nil
}
