package manager

import (
	"context"
	"errors"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
	keystoreopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	pluginHelpers "github.com/openkcm/cmk/utils/plugins"
)

type TenantConfigManager struct {
	repo         repo.Repo
	catalog      *plugincatalog.Catalog
	keystorePool *Pool
}

func NewTenantConfigManager(
	repo repo.Repo,
	catalog *plugincatalog.Catalog,
) *TenantConfigManager {
	return &TenantConfigManager{
		repo:         repo,
		catalog:      catalog,
		keystorePool: NewPool(repo),
	}
}

var (
	ErrMarshalConfig       = errors.New("error marshalling tenant config")
	ErrUnmarshalConfig     = errors.New("error unmarshalling tenant config")
	ErrGetDefaultKeystore  = errors.New("failed to get default keystore")
	ErrSetDefaultKeystore  = errors.New("failed to set default keystore")
	ErrGetKeystoreFromPool = errors.New("failed to get keystore config from pool")
)

type HYOKKeystore struct {
	Provider []string `json:"provider"`
	Allow    bool
}

type TenantKeystores struct {
	Default model.DefaultKeystore
	HYOK    HYOKKeystore
}

func (m *TenantConfigManager) GetTenantsKeystores() (TenantKeystores, error) {
	defaultKeystore := model.DefaultKeystore{}

	return TenantKeystores{
		Default: defaultKeystore,
		HYOK:    m.getTenantConfigsHyokKeystore(),
	}, nil
}

// GetDefaultKeystore retrieves the default keystore config
// If the config doesn't exist, it gets the config from the pool and sets it
func (m *TenantConfigManager) GetDefaultKeystore(ctx context.Context) (*model.KeystoreConfiguration, error) {
	var config model.TenantConfig

	ck := repo.NewCompositeKey().Where(repo.KeyField, constants.DefaultKeyStore)
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(ck),
	)

	found, err := m.repo.First(ctx, &config, *query)
	if err != nil && !found {
		var configFromPool *model.KeystoreConfiguration

		err = m.repo.Transaction(ctx, func(ctx context.Context, _ repo.Repo) error {
			var innerErr error

			configFromPool, innerErr = m.getKeystoreConfigFromPool(ctx)
			if innerErr != nil {
				return innerErr
			}

			innerErr = m.setDefaultKeystore(ctx, configFromPool)
			if innerErr != nil {
				return innerErr
			}

			return nil
		})
		if err != nil {
			return nil, err
		}

		return configFromPool, nil
	}

	if err != nil {
		return nil, errs.Wrap(ErrGetDefaultKeystore, err)
	}

	// Convert TenantConfig to KeystoreConfiguration
	keystoreConfig := &model.KeystoreConfiguration{
		Value: config.Value,
	}

	return keystoreConfig, nil
}

// SetDefaultKeystore stores the default keystore config
func (m *TenantConfigManager) setDefaultKeystore(ctx context.Context, ksConfig *model.KeystoreConfiguration) error {
	conf := &model.TenantConfig{
		Key:   constants.DefaultKeyStore,
		Value: ksConfig.Value,
	}

	err := m.repo.Set(ctx, conf)
	if err != nil {
		return errs.Wrap(ErrSetDefaultKeystore, err)
	}

	return nil
}

func (m *TenantConfigManager) getTenantConfigsHyokKeystore() HYOKKeystore {
	if m.catalog == nil {
		return HYOKKeystore{}
	}

	plugins := m.catalog.LookupByType(keystoreopv1.Type)
	if len(plugins) == 0 {
		return HYOKKeystore{}
	}

	providers := make([]string, 0)

	for _, plugin := range plugins {
		if pluginHelpers.HasTag(plugin.Info().Tags(), constants.KeyTypeHYOK) {
			providers = append(providers, plugin.Info().Name())
		}
	}

	return HYOKKeystore{Provider: providers, Allow: len(providers) > 0}
}

func (m *TenantConfigManager) getKeystoreConfigFromPool(ctx context.Context) (*model.KeystoreConfiguration, error) {
	cfg, err := m.keystorePool.Pop(ctx)
	if err != nil {
		return nil, errs.Wrap(ErrGetKeystoreFromPool, err)
	}

	return cfg, nil
}
