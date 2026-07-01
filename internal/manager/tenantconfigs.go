package manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"

	tenantpb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/repo"
	pluginHelpers "github.com/openkcm/cmk/utils/plugins"
)

const (

	// Since the workflow expiry must be less than the retention minus a day
	minimumRetentionPeriodDays = 30
	allowBYOKFeatureGateKey    = "allow-byok"
)

// Tenant config "type" values used to group flat rows in tenant_configs.
const (
	tenantConfigTypeWorkflow        = "workflow"
	tenantConfigTypeDefaultKeystore = "default_keystore"
)

// Flat-row keys for workflow config under type = "workflow".
const (
	workflowKeyEnabled                 = "enabled"
	workflowKeyMinimumApprovals        = "minimum_approvals"
	workflowKeyRetentionPeriodDays     = "retention_period_days"
	workflowKeyDefaultExpiryPeriodDays = "default_expiry_period_days"
	workflowKeyMaxExpiryPeriodDays     = "max_expiry_period_days"
)

// Flat-row keys for default keystore config under type = "default_keystore".
// LocalityID, CommonName and AccessData mirror RoleManagementConfig fields;
// KeyManagementConfig and CryptoAccessData are stored as single JSON sub-blobs.
const (
	keystoreKeyLocalityID           = "locality_id"
	keystoreKeyCommonName           = "common_name"
	keystoreKeyManagementAccessData = "management_access_data"
	keystoreKeyKeyManagementConfig  = "key_management_config"
	keystoreKeyCryptoAccessData     = "crypto_access_data"
	keystoreKeySupportedRegions     = "supported_regions"
)

var (
	ErrGetDefaultCerts = errors.New("failed to get default certificates")
	ErrDecodingCert    = errors.New("failed to decode certificate")
)

type TenantConfigManager struct {
	repo         repo.Repo
	svcRegistry  serviceapi.Registry
	keystorePool *Pool
	cfg          *config.Config
}

func NewTenantConfigManager(
	repo repo.Repo,
	svcRegistry serviceapi.Registry,
	deploymentConfig *config.Config,
) *TenantConfigManager {
	return &TenantConfigManager{
		repo:         repo,
		svcRegistry:  svcRegistry,
		keystorePool: NewPool(repo),
		cfg:          deploymentConfig,
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
	ErrWorkflowEnableDisableNotAllowed = errors.New("workflow enable/disable is only allowed for ROLE_TEST tenants")
	ErrDefaultExpiryExceedsMax         = errors.New("defaultExpiryPeriodDays must be" +
		" less than or equal to maxExpiryPeriodDays")
	ErrMinimumApprovalsTooLow = errors.New("minimumApprovals must be at least 2")
)

type HYOKKeystore struct {
	Provider []string `json:"provider"`
	Allow    bool
}

type TenantKeystores struct {
	BYOK      model.KeystoreConfig
	AllowBYOK bool
	HYOK      HYOKKeystore
}

// GetWorkflowConfig reads flat rows first, falling back to the legacy JSON
// blob for tenants whose data migration has not yet completed.
func (m *TenantConfigManager) GetWorkflowConfig(ctx context.Context) (*model.WorkflowConfig, error) {
	wc, found, err := m.getWorkflowConfigFromFlatRows(ctx)
	if err != nil {
		return nil, errs.Wrap(ErrGetWorkflowConfig, err)
	}
	if found {
		return wc, nil
	}

	wc, found, err = m.getWorkflowConfigFromLegacyBlob(ctx)
	if err != nil {
		return nil, errs.Wrap(ErrGetWorkflowConfig, err)
	}
	if found {
		return wc, nil
	}

	return m.SetWorkflowConfig(ctx, nil)
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

		workflowConfig = m.getDefaultWorkflowConfig(defaultEnabled)
	}

	if workflowConfig.RetentionPeriodDays < minimumRetentionPeriodDays {
		return nil, errs.Wrap(ErrSetWorkflowConfig, ErrRetentionLessThanMinimum)
	}

	if workflowConfig.DefaultExpiryPeriodDays > workflowConfig.MaxExpiryPeriodDays {
		return nil, errs.Wrap(ErrSetWorkflowConfig, ErrDefaultExpiryExceedsMax)
	}

	if workflowConfig.MinimumApprovals < 2 {
		return nil, errs.Wrap(ErrSetWorkflowConfig, ErrMinimumApprovalsTooLow)
	}

	if err := m.writeWorkflowConfigFlatRows(ctx, workflowConfig); err != nil {
		return nil, errs.Wrap(ErrSetWorkflowConfig, err)
	}

	return workflowConfig, nil
}

// UpdateWorkflowConfig retrieves existing config, merges with updates, and saves
func (m *TenantConfigManager) UpdateWorkflowConfig(
	ctx context.Context,
	update *cmkapi.TenantWorkflowConfiguration,
) (*model.WorkflowConfig, error) {
	// Get existing configuration
	existingConfig, err := m.GetWorkflowConfig(ctx)
	if err != nil {
		return nil, err
	}

	// If trying to change the Enabled field, validate tenant role
	if update != nil && update.Enabled != nil && *update.Enabled != existingConfig.Enabled {
		t, err := repo.GetTenant(ctx, m.repo)
		if err != nil {
			return nil, err
		}

		// Only ROLE_TEST tenants can enable/disable workflows
		if string(t.Role) != tenantpb.Role_ROLE_TEST.String() {
			return nil, errs.Wrap(ErrSetWorkflowConfig, ErrWorkflowEnableDisableNotAllowed)
		}
	}

	// Merge the update with existing config
	mergedConfig := m.mergeWorkflowConfig(existingConfig, update)

	// Save and return the updated configuration
	return m.SetWorkflowConfig(ctx, mergedConfig)
}

func (m *TenantConfigManager) GetTenantsKeystores(ctx context.Context) (TenantKeystores, error) {
	defaultKeystore, found, err := m.GetStoredDefaultKeystoreConfig(ctx)
	if err != nil {
		return TenantKeystores{}, err
	}

	byokKeystore := model.KeystoreConfig{}
	if found {
		byokKeystore = *defaultKeystore
	}

	return TenantKeystores{
		BYOK:      byokKeystore,
		AllowBYOK: m.isBYOKAllowed(),
		HYOK:      m.getTenantConfigsHyokKeystore(),
	}, nil
}

// GetDefaultKeystoreConfig retrieves the default keystore config
// If the config doesn't exist, it gets the config from the pool and sets it
func (m *TenantConfigManager) GetDefaultKeystoreConfig(ctx context.Context) (*model.KeystoreConfig, error) {
	keystore, found, err := m.GetStoredDefaultKeystoreConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !found {
		err = m.repo.Transaction(ctx, func(ctx context.Context) error {
			keystore, err = m.getKeystoreConfigFromPool(ctx)
			if err != nil {
				return err
			}

			err = m.SetDefaultKeystore(ctx, keystore)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return nil, err
		}

		return keystore, nil
	}

	return keystore, nil
}

// GetStoredDefaultKeystoreConfig reads the stored default keystore without
// pool fallback. Reads flat rows first, falling back to the legacy JSON blob.
func (m *TenantConfigManager) GetStoredDefaultKeystoreConfig(ctx context.Context) (*model.KeystoreConfig, bool, error) {
	ks, found, err := m.getKeystoreConfigFromFlatRows(ctx)
	if err != nil {
		return nil, false, errs.Wrap(ErrGetDefaultKeystore, err)
	}
	if found {
		return ks, true, nil
	}

	return m.getKeystoreConfigFromLegacyBlob(ctx)
}

// SetDefaultKeystore stores the default keystore config
func (m *TenantConfigManager) SetDefaultKeystore(ctx context.Context, keystore *model.KeystoreConfig) error {
	if err := m.writeKeystoreConfigFlatRows(ctx, keystore); err != nil {
		return errs.Wrap(ErrSetDefaultKeystore, err)
	}

	return nil
}

// isBYOKAllowed checks whether BYOK is enabled by deployment feature-gate configuration.
func (m *TenantConfigManager) isBYOKAllowed() bool {
	if m.cfg == nil {
		return false
	}

	return m.cfg.FeatureGates.IsFeatureEnabled(allowBYOKFeatureGateKey)
}

func (m *TenantConfigManager) getTenantConfigsHyokKeystore() HYOKKeystore {
	if m.svcRegistry == nil {
		return HYOKKeystore{}
	}

	plugins, err := m.svcRegistry.KeyManagementList()
	if err != nil || len(plugins) == 0 {
		return HYOKKeystore{}
	}

	providers := make([]string, 0)

	for _, plugin := range plugins {
		if pluginHelpers.HasTag(plugin.ServiceInfo().Tags(), constants.KeyTypeHYOK) {
			providers = append(providers, plugin.ServiceInfo().Name())
		}
	}

	if len(providers) == 0 {
		return HYOKKeystore{}
	}

	sort.Strings(providers)

	return HYOKKeystore{Provider: providers, Allow: true}
}

func (m *TenantConfigManager) getKeystoreConfigFromPool(ctx context.Context) (*model.KeystoreConfig, error) {
	cfg, err := m.keystorePool.Pop(ctx)
	if err != nil {
		return nil, errs.Wrap(ErrGetKeystoreFromPool, err)
	}

	ksConfig := &model.KeystoreConfig{}

	err = json.Unmarshal(cfg.Config, ksConfig)
	if err != nil {
		return nil, errs.Wrap(ErrUnmarshalConfig, err)
	}

	return ksConfig, nil
}

func (m *TenantConfigManager) getWorkflowConfigFromFlatRows(
	ctx context.Context,
) (*model.WorkflowConfig, bool, error) {
	configs, err := m.listConfigsByType(ctx, tenantConfigTypeWorkflow)
	if err != nil {
		return nil, false, err
	}
	if len(configs) == 0 {
		return nil, false, nil
	}

	return buildWorkflowConfigFromRows(configs)
}

// requiredWorkflowKeys are the keys that must be present for the flat-row
// workflow config to be considered complete; otherwise the caller falls back
// to the legacy blob.
var requiredWorkflowKeys = []string{
	workflowKeyEnabled,
	workflowKeyMinimumApprovals,
	workflowKeyRetentionPeriodDays,
	workflowKeyDefaultExpiryPeriodDays,
	workflowKeyMaxExpiryPeriodDays,
}

func buildWorkflowConfigFromRows(configs []model.TenantConfig) (*model.WorkflowConfig, bool, error) {
	wc := &model.WorkflowConfig{}
	seen := make(map[string]struct{}, len(requiredWorkflowKeys))

	for _, c := range configs {
		if err := applyWorkflowConfigField(wc, c.Key, c.Value); err != nil {
			return nil, false, err
		}
		seen[c.Key] = struct{}{}
	}

	for _, k := range requiredWorkflowKeys {
		if _, ok := seen[k]; !ok {
			return nil, false, nil
		}
	}

	return wc, true, nil
}

//nolint:cyclop // simple switch over a fixed set of keys
func applyWorkflowConfigField(wc *model.WorkflowConfig, key, value string) error {
	switch key {
	case workflowKeyEnabled:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("parse %s: %w", key, err)
		}
		wc.Enabled = b
	case workflowKeyMinimumApprovals:
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse %s: %w", key, err)
		}
		wc.MinimumApprovals = v
	case workflowKeyRetentionPeriodDays:
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse %s: %w", key, err)
		}
		wc.RetentionPeriodDays = v
	case workflowKeyDefaultExpiryPeriodDays:
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse %s: %w", key, err)
		}
		wc.DefaultExpiryPeriodDays = v
	case workflowKeyMaxExpiryPeriodDays:
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse %s: %w", key, err)
		}
		wc.MaxExpiryPeriodDays = v
	}

	return nil
}

func (m *TenantConfigManager) writeWorkflowConfigFlatRows(
	ctx context.Context,
	wc *model.WorkflowConfig,
) error {
	t := tenantConfigTypeWorkflow
	rows := []model.TenantConfig{
		{Key: workflowKeyEnabled, Value: strconv.FormatBool(wc.Enabled), Type: t},
		{Key: workflowKeyMinimumApprovals, Value: strconv.Itoa(wc.MinimumApprovals), Type: t},
		{Key: workflowKeyRetentionPeriodDays, Value: strconv.Itoa(wc.RetentionPeriodDays), Type: t},
		{Key: workflowKeyDefaultExpiryPeriodDays, Value: strconv.Itoa(wc.DefaultExpiryPeriodDays), Type: t},
		{Key: workflowKeyMaxExpiryPeriodDays, Value: strconv.Itoa(wc.MaxExpiryPeriodDays), Type: t},
	}

	return m.setRows(ctx, rows)
}

func (m *TenantConfigManager) getKeystoreConfigFromFlatRows(
	ctx context.Context,
) (*model.KeystoreConfig, bool, error) {
	configs, err := m.listConfigsByType(ctx, tenantConfigTypeDefaultKeystore)
	if err != nil {
		return nil, false, err
	}
	if len(configs) == 0 {
		return nil, false, nil
	}

	return buildKeystoreConfigFromRows(configs)
}

// buildKeystoreConfigFromRows returns found=false when required identity fields
// are missing, so the caller can fall back to the legacy blob.
func buildKeystoreConfigFromRows(configs []model.TenantConfig) (*model.KeystoreConfig, bool, error) {
	ks := &model.KeystoreConfig{}

	for _, c := range configs {
		if err := applyKeystoreConfigField(ks, c.Key, c.Value); err != nil {
			return nil, false, err
		}
	}

	if ks.RoleManagementConfig.LocalityID == "" || ks.RoleManagementConfig.CommonName == "" {
		return nil, false, nil
	}

	return ks, true, nil
}

//nolint:cyclop // simple switch over a fixed set of keys
func applyKeystoreConfigField(ks *model.KeystoreConfig, key, value string) error {
	switch key {
	case keystoreKeyLocalityID:
		ks.RoleManagementConfig.LocalityID = value
	case keystoreKeyCommonName:
		ks.RoleManagementConfig.CommonName = value
	case keystoreKeyManagementAccessData:
		var ad model.KeystoreAccessData
		if err := json.Unmarshal([]byte(value), &ad); err != nil {
			return errs.Wrap(ErrUnmarshalConfig, err)
		}
		ks.RoleManagementConfig.AccessData = ad
	case keystoreKeyKeyManagementConfig:
		if err := json.Unmarshal([]byte(value), &ks.KeyManagementConfig); err != nil {
			return errs.Wrap(ErrUnmarshalConfig, err)
		}
	case keystoreKeyCryptoAccessData:
		if err := json.Unmarshal([]byte(value), &ks.CryptoAccessData); err != nil {
			return errs.Wrap(ErrUnmarshalConfig, err)
		}
	case keystoreKeySupportedRegions:
		if err := json.Unmarshal([]byte(value), &ks.SupportedRegions); err != nil {
			return errs.Wrap(ErrUnmarshalConfig, err)
		}
	}

	return nil
}

// writeKeystoreConfigFlatRows replaces the default-keystore flat rows
// (delete + insert in one tx) so omitted optional fields don't leave stale
// rows behind — matching the legacy blob's whole-object replace semantics.
func (m *TenantConfigManager) writeKeystoreConfigFlatRows(
	ctx context.Context,
	ks *model.KeystoreConfig,
) error {
	t := tenantConfigTypeDefaultKeystore
	rows := []model.TenantConfig{
		{Key: keystoreKeyLocalityID, Value: ks.RoleManagementConfig.LocalityID, Type: t},
		{Key: keystoreKeyCommonName, Value: ks.RoleManagementConfig.CommonName, Type: t},
	}

	if ks.RoleManagementConfig.AccessData != nil {
		adBytes, err := json.Marshal(ks.RoleManagementConfig.AccessData)
		if err != nil {
			return errs.Wrap(ErrMarshalConfig, err)
		}
		rows = append(rows, model.TenantConfig{
			Key: keystoreKeyManagementAccessData, Value: string(adBytes), Type: t,
		})
	}

	// KeyManagementConfig is stored as a single JSON sub-blob.
	kmBytes, err := json.Marshal(ks.KeyManagementConfig)
	if err != nil {
		return errs.Wrap(ErrMarshalConfig, err)
	}
	rows = append(rows, model.TenantConfig{
		Key: keystoreKeyKeyManagementConfig, Value: string(kmBytes), Type: t,
	})

	if ks.CryptoAccessData != nil {
		cdBytes, err := json.Marshal(ks.CryptoAccessData)
		if err != nil {
			return errs.Wrap(ErrMarshalConfig, err)
		}
		rows = append(rows, model.TenantConfig{
			Key: keystoreKeyCryptoAccessData, Value: string(cdBytes), Type: t,
		})
	}

	if ks.SupportedRegions != nil {
		regBytes, err := json.Marshal(ks.SupportedRegions)
		if err != nil {
			return errs.Wrap(ErrMarshalConfig, err)
		}
		rows = append(rows, model.TenantConfig{
			Key: keystoreKeySupportedRegions, Value: string(regBytes), Type: t,
		})
	}

	return m.replaceRowsByType(ctx, t, rows)
}

func (m *TenantConfigManager) getWorkflowConfigFromLegacyBlob(
	ctx context.Context,
) (*model.WorkflowConfig, bool, error) {
	tc, found, err := m.getLegacyBlob(ctx, constants.WorkflowConfigKey)
	if err != nil || !found {
		return nil, found, err
	}

	var wc model.WorkflowConfig
	if err := json.Unmarshal([]byte(tc.Value), &wc); err != nil {
		return nil, false, errs.Wrap(ErrUnmarshalConfig, err)
	}

	return &wc, true, nil
}

func (m *TenantConfigManager) getKeystoreConfigFromLegacyBlob(
	ctx context.Context,
) (*model.KeystoreConfig, bool, error) {
	tc, found, err := m.getLegacyBlob(ctx, constants.DefaultKeyStore)
	if err != nil || !found {
		return nil, found, err
	}

	keystore := &model.KeystoreConfig{}
	if err := json.Unmarshal([]byte(tc.Value), keystore); err != nil {
		return nil, false, errs.Wrap(ErrUnmarshalConfig, err)
	}

	return keystore, true, nil
}

func (m *TenantConfigManager) getLegacyBlob(
	ctx context.Context,
	key string,
) (*model.TenantConfig, bool, error) {
	var tc model.TenantConfig

	ck := repo.NewCompositeKey().
		Where(repo.KeyField, key).
		Where(repo.TypeField, "")
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))

	found, err := m.repo.First(ctx, &tc, *query)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}

	return &tc, true, nil
}

func (m *TenantConfigManager) listConfigsByType(
	ctx context.Context,
	configType string,
) ([]model.TenantConfig, error) {
	var configs []model.TenantConfig

	ck := repo.NewCompositeKey().Where(repo.TypeField, configType)
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))

	if err := m.repo.List(ctx, &model.TenantConfig{}, &configs, *query); err != nil {
		// Preserve the same error contract as the legacy First-based path so that
		// API error mappings keyed on repo.ErrGetResource keep working.
		return nil, errs.Wrap(repo.ErrGetResource, err)
	}

	return configs, nil
}

// setRows upserts a slice of TenantConfig rows in a single transaction.
func (m *TenantConfigManager) setRows(ctx context.Context, rows []model.TenantConfig) error {
	return m.repo.Transaction(ctx, func(ctx context.Context) error {
		for i := range rows {
			if err := m.repo.Set(ctx, &rows[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

// replaceRowsByType deletes all rows of the given type and inserts the
// provided rows in a single transaction.
func (m *TenantConfigManager) replaceRowsByType(
	ctx context.Context,
	configType string,
	rows []model.TenantConfig,
) error {
	return m.repo.Transaction(ctx, func(ctx context.Context) error {
		ck := repo.NewCompositeKey().Where(repo.TypeField, configType)
		query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))
		if _, err := m.repo.Delete(ctx, &model.TenantConfig{}, *query); err != nil {
			return err
		}
		for i := range rows {
			if err := m.repo.Set(ctx, &rows[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

// getDefaultWorkflowConfig returns default workflow config, checking deploymentConfig first,
// then falling back to hard-coded constants
func (m *TenantConfigManager) getDefaultWorkflowConfig(defaultEnabled bool) *model.WorkflowConfig {
	c := &model.WorkflowConfig{
		Enabled:                 defaultEnabled,
		MinimumApprovals:        constants.DefaultMinimumApprovalCount,
		RetentionPeriodDays:     constants.DefaultRetentionPeriodDays,
		DefaultExpiryPeriodDays: constants.DefaultExpiryPeriodDays,
		MaxExpiryPeriodDays:     constants.DefaultMaxExpiryPeriodDays,
	}

	// Override with deploymentConfig values if available
	if m.cfg == nil {
		return c
	}

	m.applyDeploymentConfigOverrides(c)
	return c
}

// applyDeploymentConfigOverrides applies deployment config values to workflow config
// to override any default values.
func (m *TenantConfigManager) applyDeploymentConfigOverrides(config *model.WorkflowConfig) {
	if m.cfg.Workflow.DefaultMinimumApprovals > 0 {
		config.MinimumApprovals = m.cfg.Workflow.DefaultMinimumApprovals
	}
	if m.cfg.Workflow.DefaultRetentionPeriodDays > 0 {
		config.RetentionPeriodDays = m.cfg.Workflow.DefaultRetentionPeriodDays
	}
	if m.cfg.Workflow.DefaultExpiryPeriodDays > 0 {
		config.DefaultExpiryPeriodDays = m.cfg.Workflow.DefaultExpiryPeriodDays
	}
	if m.cfg.Workflow.DefaultMaxExpiryPeriodDays > 0 {
		config.MaxExpiryPeriodDays = m.cfg.Workflow.DefaultMaxExpiryPeriodDays
	}
}

// mergeWorkflowConfig merges partial updates into existing config
func (m *TenantConfigManager) mergeWorkflowConfig(
	existing *model.WorkflowConfig,
	update *cmkapi.TenantWorkflowConfiguration,
) *model.WorkflowConfig {
	if update == nil {
		return existing
	}

	// Start with existing config or create new one
	result := &model.WorkflowConfig{}
	if existing != nil {
		*result = *existing
	}

	// Apply updates (merge-patch semantics)
	if update.Enabled != nil {
		result.Enabled = *update.Enabled
	}
	if update.MinimumApprovals != nil {
		result.MinimumApprovals = *update.MinimumApprovals
	}
	if update.RetentionPeriodDays != nil {
		result.RetentionPeriodDays = *update.RetentionPeriodDays
	}
	if update.DefaultExpiryPeriodDays != nil {
		result.DefaultExpiryPeriodDays = *update.DefaultExpiryPeriodDays
	}
	if update.MaxExpiryPeriodDays != nil {
		result.MaxExpiryPeriodDays = *update.MaxExpiryPeriodDays
	}

	return result
}
