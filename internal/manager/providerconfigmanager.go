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
	"google.golang.org/protobuf/types/known/structpb"

	kscommonv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/common/v1"
	keystoremanagerv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/management/v1"
	keystoreopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	cmkplugincatalog "github.com/openkcm/cmk/internal/plugincatalog"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	pluginHelpers "github.com/openkcm/cmk/utils/plugins"
	"github.com/openkcm/cmk/utils/ptr"
)

const (
	DefaultHYOKCertCommonName            = "default.hyok.cmk"
	DefaultProviderConfigCacheExpiration = 24 * time.Hour
)

var (
	ErrCreateKeystore                = errors.New("failed to create keystore")
	ErrInvalidKeystore               = errors.New("invalid keystore")
	ErrCreateProtobufStruct          = errors.New("failed to create protobuf struct")
	ErrGetTenantFromCtx              = errors.New("failed to get tenant from context")
	ErrGetDefaultTenantCertificate   = errors.New("failed to get default tenant HYOK certificate")
	ErrGetDefaultKeystoreCertificate = errors.New("failed to get default keystore certificate")
	ErrAddConfigToPool               = errors.New("failed to add keystore configuration to pool")
	ErrCountKeystorePool             = errors.New("failed to get keystore pool size")
)

type ProviderConfig struct {
	Config     *kscommonv1.KeystoreInstanceConfig
	Client     keystoreopv1.KeystoreInstanceKeyOperationClient
	Expiration time.Time // Optional expiration time for the provider config
}

func NewProviderConfig(
	config *kscommonv1.KeystoreInstanceConfig,
	client keystoreopv1.KeystoreInstanceKeyOperationClient,
	expiration *time.Time,
) *ProviderConfig {
	if expiration == nil {
		expiration = ptr.PointTo(time.Now().Add(DefaultProviderConfigCacheExpiration)) // Default expiration if nil
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
	catalog       *cmkplugincatalog.Registry
	providers     map[ProviderCachedKey]*ProviderConfig
	mu            sync.RWMutex
	tenantConfigs *TenantConfigManager
	certs         *CertificateManager
	repo          repo.Repo
	keystorePool  *Pool
}

const (
	pluginAlgorithmPrefix = "KEY_ALGORITHM_"
	pluginKeyTypePrefix   = "KEY_TYPE_"
)

// getPluginAlgorithm returns the plugin algorithm for the key
func getPluginAlgorithm(alg string) string {
	return pluginAlgorithmPrefix + alg
}

// getPluginKeyType returns the plugin key type for the key
func getPluginKeyType(keyType string) string {
	return pluginKeyTypePrefix + keyType
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
	if key.KeyType == constants.KeyTypeHYOK {
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
		slog.String("KeyStore", keystoreName),
		slog.String("Provider", provider),
	)

	config, expiration, err := pmc.getKeystoreConfig(ctx, keystoreName)
	if err != nil {
		return nil, errs.Wrap(ErrConfigNotFound, err)
	}

	// Initialize client
	plugin := pmc.catalog.LookupByTypeAndName(keystoreopv1.Type, provider)
	if plugin == nil {
		return nil, errs.Wrapf(ErrPluginNotFound, provider)
	}

	client := keystoreopv1.NewKeystoreInstanceKeyOperationClient(plugin.ClientConnection())

	providerCfg := NewProviderConfig(config, client, expiration)

	pmc.providers[compositeKey] = providerCfg

	return providerCfg, nil
}

func (pmc *ProviderConfigManager) FillKeystorePool(ctx context.Context, size int) error {
	count, err := pmc.keystorePool.Count(ctx)
	if err != nil {
		return errs.Wrap(ErrCountKeystorePool, err)
	}

	log.Debug(ctx, "Filling keystore pool",
		slog.Int("Current Size", count),
		slog.Int("Target Size", size),
	)

	for i := count; i < size; i++ {
		provider, config, err := pmc.CreateKeystore(ctx)
		if err != nil {
			return err
		}

		err = pmc.AddKeystoreToPool(ctx, provider, config)
		if err != nil {
			return err
		}
	}

	log.Debug(ctx, "Keystore Pool Filled",
		slog.Int("New Size", size),
	)

	return nil
}

func (pmc *ProviderConfigManager) CreateKeystore(ctx context.Context) (string, map[string]any, error) {
	provider, err := pmc.GetDefaultKeystoreFromCatalog()
	if err != nil {
		return "", nil, err
	}

	plugin := pmc.catalog.LookupByTypeAndName(keystoremanagerv1.Type, provider)
	if plugin == nil {
		return "", nil, errs.Wrapf(ErrPluginNotFound, provider)
	}

	client := keystoremanagerv1.NewKeystoreProviderClient(plugin.ClientConnection())

	resp, err := client.CreateKeystore(ctx, &keystoremanagerv1.CreateKeystoreRequest{})
	if err != nil {
		return "", nil, errs.Wrapf(ErrCreateKeystore, fmt.Sprintf("provider: %s, error: %v", provider, err))
	}

	configValues := resp.GetConfig().GetValues()

	return provider, configValues.AsMap(), nil
}

func (pmc *ProviderConfigManager) AddKeystoreToPool(
	ctx context.Context,
	provider string,
	config map[string]any,
) error {
	ksConfig, err := json.Marshal(config)
	if err != nil {
		return errs.Wrap(ErrMarshalConfig, err)
	}

	_, err = pmc.keystorePool.Add(ctx, &model.Keystore{
		ID:       uuid.New(),
		Provider: provider,
		Config:   ksConfig,
	})
	if err != nil {
		return errs.Wrap(ErrAddConfigToPool, err)
	}

	return nil
}

func (pmc *ProviderConfigManager) GetDefaultKeystoreFromCatalog() (string, error) {
	if pmc.catalog == nil {
		return "", errs.Wrapf(ErrGetDefaultKeystore, "no plugin catalog available")
	}

	plugins := pmc.catalog.LookupByType(keystoreopv1.Type)
	if len(plugins) == 0 {
		return "", errs.Wrapf(ErrGetDefaultKeystore, "no keystore plugins found in catalog")
	}

	providers := make([]string, 0)

	for _, plugin := range plugins {
		if pluginHelpers.HasTag(plugin.Info().Tags(), constants.DefaultKeyStore) {
			providers = append(providers, plugin.Info().Name())
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
) (*kscommonv1.KeystoreInstanceConfig, *time.Time, error) {
	switch keystoreName {
	case constants.DefaultKeyStore:
		return pmc.getDefaultKeystoreConfig(ctx)
	case constants.HYOKKeyStore:
		return pmc.getHYOKKeystoreConfig(ctx)
	default:
		return nil, nil, ErrInvalidKeystore
	}
}

func (pmc *ProviderConfigManager) createKeystoreInstanceConfig(
	configMap map[string]any,
) (*kscommonv1.KeystoreInstanceConfig, error) {
	config, err := structpb.NewStruct(configMap)
	if err != nil {
		return nil, errs.Wrap(ErrCreateProtobufStruct, err)
	}

	return &kscommonv1.KeystoreInstanceConfig{Values: config}, nil
}

func (pmc *ProviderConfigManager) getDefaultKeystoreConfig(
	ctx context.Context,
) (*kscommonv1.KeystoreInstanceConfig, *time.Time, error) {
	ksConfig, err := pmc.tenantConfigs.GetDefaultKeystoreConfig(ctx)
	if err != nil {
		return nil, nil, err
	}

	cert, err := pmc.certs.getDefaultKeystoreClientCert(
		ctx,
		ksConfig.LocalityID,
		ksConfig.CommonName,
	)
	if err != nil {
		return nil, nil, err
	}

	configMap := map[string]any{
		"authType":   constants.AuthTypeCertificate,
		"clientCert": cert.CertPEM,
		"privateKey": cert.PrivateKeyPEM,
	}

	maps.Copy(configMap, ksConfig.ManagementAccessData)

	config, err := pmc.createKeystoreInstanceConfig(configMap)
	if err != nil {
		return nil, nil, err
	}

	return config, &cert.ExpirationDate, nil
}

func (pmc *ProviderConfigManager) getHYOKKeystoreConfig(
	ctx context.Context,
) (*kscommonv1.KeystoreInstanceConfig, *time.Time, error) {
	cert, err := pmc.certs.getDefaultHYOKClientCert(ctx)
	if err != nil {
		return nil, nil, err
	}

	configMap := map[string]any{
		"authType":   constants.AuthTypeCertificate,
		"clientCert": cert.CertPEM,
		"privateKey": cert.PrivateKeyPEM,
	}

	config, err := pmc.createKeystoreInstanceConfig(configMap)
	if err != nil {
		return nil, nil, err
	}

	return config, &cert.ExpirationDate, nil
}
