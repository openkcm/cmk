package manager

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	tenantpb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
	keystoreopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"

	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	pluginHelpers "github.tools.sap/kms/cmk/utils/plugins"
)

// Since the workflow expiry must be less than the retention minus a day
const minimumRetentionPeriodDays = 2

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
	ErrMarshalConfig            = errors.New("error marshalling tenant config")
	ErrUnmarshalConfig          = errors.New("error unmarshalling tenant config")
	ErrGetDefaultKeystore       = errors.New("failed to get default keystore")
	ErrSetDefaultKeystore       = errors.New("failed to set default keystore")
	ErrGetKeystoreFromPool      = errors.New("failed to get keystore config from pool")
	ErrGetWorkflowConfig        = errors.New("failed to get workflow config")
	ErrSetWorkflowConfig        = errors.New("failed to set workflow config")
	ErrRetentionLessThanMinimum = errors.New("retention is less than the minimum allowed (" +
		strconv.Itoa(minimumRetentionPeriodDays) + " day)")
)

type HYOKKeystore struct {
	Provider []string `json:"provider"`
	Allow    bool
}

type TenantKeystores struct {
	Default model.DefaultKeystore
	HYOK    HYOKKeystore
}

func (m *TenantConfigManager) GetWorkflowConfig(ctx context.Context) (*model.WorkflowConfig, error) {
	var config model.TenantConfig

	ck := repo.NewCompositeKey().Where(repo.KeyField, constants.WorkflowConfigKey)
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(ck),
	)

	found, err := m.repo.First(ctx, &config, *query)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, errs.Wrap(ErrGetWorkflowConfig, err)
	}

	if !found {
		return m.SetWorkflowConfig(ctx, nil)
	}

	// Convert TenantConfig to WorkflowConfig
	workflowConfig, err := m.convertToWorkflowConfig(&config)
	if err != nil {
		return nil, errs.Wrap(ErrUnmarshalConfig, err)
	}

	return workflowConfig, nil
}

// SetWorkflowConfig stores the workflow config or creates default if nil
func (m *TenantConfigManager) SetWorkflowConfig(
	ctx context.Context,
	workflowConfig *model.WorkflowConfig,
) (*model.WorkflowConfig, error) {
	// If no config provided, create default based on tenant role
	if workflowConfig == nil {
		t, err := repo.GetTenant(ctx, m.repo)
		if err != nil {
			return nil, err
		}

		defaultEnabled := false
		if string(t.Role) == tenantpb.Role_ROLE_LIVE.String() {
			defaultEnabled = true
		}

		defaultMinimumApprovalCount := 2
		defaultRetentionPeriodDays := 30
		defaultDefaultExpiryPeriodDays := 7
		defaultMaxExpiryPeriodDays := 30

		workflowConfig = &model.WorkflowConfig{
			Enabled:                 defaultEnabled,
			MinimumApprovals:        defaultMinimumApprovalCount,
			RetentionPeriodDays:     defaultRetentionPeriodDays,
			DefaultExpiryPeriodDays: defaultDefaultExpiryPeriodDays,
			MaxExpiryPeriodDays:     defaultMaxExpiryPeriodDays,
		}
	}

	if workflowConfig.RetentionPeriodDays < minimumRetentionPeriodDays {
		return nil, errs.Wrap(ErrSetWorkflowConfig, ErrRetentionLessThanMinimum)
	}

	configValue, err := json.Marshal(workflowConfig)
	if err != nil {
		return nil, errs.Wrap(ErrMarshalConfig, err)
	}

	conf := &model.TenantConfig{
		Key:   constants.WorkflowConfigKey,
		Value: configValue,
	}

	err = m.repo.Set(ctx, conf)
	if err != nil {
		return nil, errs.Wrap(ErrSetWorkflowConfig, err)
	}

	return workflowConfig, nil
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
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, errs.Wrap(ErrGetDefaultKeystore, err)
	}

	if !found {
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

// convertToWorkflowConfig converts TenantConfig to WorkflowConfig
func (m *TenantConfigManager) convertToWorkflowConfig(config *model.TenantConfig) (*model.WorkflowConfig, error) {
	var workflowConfig model.WorkflowConfig

	err := json.Unmarshal(config.Value, &workflowConfig)
	if err != nil {
		return nil, err
	}

	return &workflowConfig, nil
}
