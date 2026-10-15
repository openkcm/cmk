package manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strconv"


	tenantpb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/common"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keystoremanagement"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	pluginHelpers "github.com/openkcm/cmk/utils/plugins"
)

const (
	// defaultKeystoreCertInfix is inserted between the tenant cert prefix and the tenantID
	// when constructing the BYOK key-management CN, keeping it under the X.509 64-char limit.
	defaultKeystoreCertInfix = "byok-"
)

var (
	ErrGetDefaultCerts = errors.New("failed to get default certificates")
	ErrDecodingCert    = errors.New("failed to decode certificate")
)

type TenantConfigManager struct {
	r            repo.Repo
	svcRegistry  serviceapi.Registry
	keystorePool *Pool
	cfg          *config.Config
	certs        *CertificateManager
	flags        FeatureFlags
}

func NewTenantConfigManager(
	repo repo.Repo,
	svcRegistry serviceapi.Registry,
	deploymentConfig *config.Config,
	certs *CertificateManager,
	flags FeatureFlags,
) *TenantConfigManager {
	return &TenantConfigManager{
		r:            repo,
		svcRegistry:  svcRegistry,
		keystorePool: NewPool(repo),
		cfg:          deploymentConfig,
		certs:        certs,
		flags:        flags,
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
	ErrGetTenantLimits          = errors.New("failed to get tenant limits")
	ErrRetentionLessThanMinimum = errors.New("retention is less than the minimum allowed (" +
		strconv.Itoa(constants.MinRetentionPeriodDays) + " days)")
	ErrRetentionExceedsMaximum = errors.New("retention exceeds the maximum allowed (" +
		strconv.Itoa(constants.MaxRetentionPeriodDays) + " days)")
	ErrWorkflowEnableDisableNotAllowed = errors.New("workflow enable/disable is only allowed for ROLE_TEST tenants")
	ErrDefaultExpiryExceedsMax         = errors.New("defaultExpiryPeriodDays must be" +
		" less than or equal to maxExpiryPeriodDays")
	ErrMinimumApprovalsTooLow  = errors.New("minimumApprovals must be at least 2")
	ErrMinimumApprovalsTooHigh = errors.New("minimumApprovals must be at most " +
		strconv.Itoa(constants.MaxMinimumApprovals))
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

func (m *TenantConfigManager) SetTenantConfig(ctx context.Context, config model.TenantConfiguration) error {
	flattened, err := model.Flatten(config)
	if err != nil {
		return err
	}
	query := repo.NewQuery().OnConflict(repo.KeyField, repo.TypeField)
	for k, v := range flattened {
		tenantConfig := &model.TenantConfig{Key: k, Value: fmt.Sprint(v), Type: config.Type()}
		err := m.r.Set(ctx, tenantConfig, *query)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetTenantConfigByType fetches from the DB properties related with a ConfigType
// The Confifguration must implement the TenantConfiguration interface
func (m *TenantConfigManager) GetTenantConfigByType[T model.TenantConfiguration](
	ctx context.Context,
) (*T,error) {
	var res T
	configs := []*model.TenantConfig{}

	ck := repo.NewCompositeKey().Where(repo.TypeField, res.Type())
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))

	if err := m.r.List(ctx, &model.TenantConfig{}, &configs, *query); err != nil {
		// Preserve the same error contract as the legacy First-based path so that
		// API error mappings keyed on repo.ErrGetResource keep working.
		return nil, errs.Wrap(repo.ErrGetResource, err)
	}

	// if len(configs) < 1 {
	// 	return nil, nil
	// }

	configMap := make(map[string]any, len(configs))
	for _, c := range configs {
		configMap[c.Key] = c.Value
	}

	res, err := model.Unflatten[T](configMap)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (m *TenantConfigManager) GetTenantLimits(ctx context.Context) (model.TenantLimits, error) {
	configs, err := m.GetTenantConfigByType[model.TenantLimits](ctx)
	if err != nil {
		return model.TenantLimits{}, errs.Wrap(ErrGetTenantLimits, err)
	}

	if m.cfg != nil {
		if configs.KeyConfigs == 0 {
			configs.KeyConfigs = m.cfg.Tenant.KeyConfigLimit
		}
		if configs.Systems == 0 {
			configs.Systems = m.cfg.Tenant.SystemLimit
		}
		if configs.Keys == 0 {
			configs.Keys = m.cfg.Tenant.KeyLimit
		}
	}

	return *configs, nil
}

// GetSystemGroupingConfig gets systemgrouping config out of tenant_config
// If it does not exist, set in the DB both as false
func (m *TenantConfigManager) GetSystemGroupingConfig(ctx context.Context) (*model.SystemGroupConfig, error) {
	configs, err := m.GetTenantConfigByType[model.SystemGroupConfig](ctx)
	if err != nil {
		return nil, err
	}


	if configs != nil {
		err = m.SetTenantConfig(ctx, model.SystemGroupConfig{
			Enabled:      false,
			WarnCoverage: false,
		})
		if err != nil {
			return nil, err
		}
	}

	return &model.SystemGroupConfig{
		Enabled:      false,
		WarnCoverage: false,
	}, nil
}

// GetWorkflowConfig reads flat rows first, falling back to the legacy JSON
// blob for tenants whose data migration has not yet completed.
func (m *TenantConfigManager) GetWorkflowConfig(ctx context.Context) (*model.WorkflowConfig, error) {
	wc, err := m.GetTenantConfigByType[model.WorkflowConfig](ctx)
	if err != nil {
		return nil, errs.Wrap(ErrGetWorkflowConfig, err)
	}

	if wc != nil {
		return wc, nil
	}


	wc, found, err := m.getWorkflowConfigFromLegacyBlob(ctx)
	if err != nil {
		return nil, errs.Wrap(ErrGetWorkflowConfig, err)
	}
	if found {
		return wc, nil
	}

	return m.SetWorkflowConfig(ctx, nil)
}

// validateWorkflowConfig checks that all workflow config fields are within hard limits.
// The bounds on retentionPeriodDays [7,30] and maxExpiryPeriodDays [1,7] together
// guarantee retention >= maxExpiry by construction.
func validateWorkflowConfig(config *model.WorkflowConfig) error {
	if config.RetentionPeriodDays < constants.MinRetentionPeriodDays {
		return ErrRetentionLessThanMinimum
	}
	if config.RetentionPeriodDays > constants.MaxRetentionPeriodDays {
		return ErrRetentionExceedsMaximum
	}
	if config.DefaultExpiryPeriodDays > config.MaxExpiryPeriodDays {
		return ErrDefaultExpiryExceedsMax
	}
	if config.MinimumApprovals < 2 {
		return ErrMinimumApprovalsTooLow
	}
	if config.MinimumApprovals > constants.MaxMinimumApprovals {
		return ErrMinimumApprovalsTooHigh
	}
	return nil
}

// SetWorkflowConfig stores the workflow config or creates default if nil
func (m *TenantConfigManager) SetWorkflowConfig(
	ctx context.Context,
	workflowConfig *model.WorkflowConfig,
) (*model.WorkflowConfig, error) {
	// If no config provided, create default based on tenant role
	if workflowConfig == nil {
		t, err := repo.GetTenant(ctx, m.r)
		if err != nil {
			return nil, err
		}

		defaultEnabled := false
		if string(t.Role) == tenantpb.Role_ROLE_LIVE.String() {
			defaultEnabled = true
		}

		workflowConfig = m.getDefaultWorkflowConfig(defaultEnabled)
	}

	if err := validateWorkflowConfig(workflowConfig); err != nil {
		return nil, errs.Wrap(ErrSetWorkflowConfig, err)
	}

	err := m.SetTenantConfig(ctx, workflowConfig)
	if err != nil {
		return nil, errs.Wrap(ErrSetWorkflowConfig, err)
	}

	return workflowConfig, nil
}

// UpdateWorkflowConfig retrieves existing config, merges with updates, and saves
func (m *TenantConfigManager) UpdateWorkflowConfig(
	ctx context.Context,
	update *cmkapi.TenantWorkflowConfiguration,
) (*model.WorkflowConfig, error) {
	existingConfig, err := m.GetWorkflowConfig(ctx)
	if err != nil {
		return nil, err
	}

	// If trying to change the Enabled field, validate tenant role
	if update != nil && update.Enabled != nil && *update.Enabled != existingConfig.Enabled {
		t, err := repo.GetTenant(ctx, m.r)
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

	byokKeystore := &model.KeystoreConfig{}
	if found {
		byokKeystore = defaultKeystore
	}
	if m.flags.IsBYOKAllowed(ctx) && m.cfg != nil {
		byokKeystore.SupportedRegions = regionsByTechnicalName(m.cfg.KeystorePool.SupportedRegions)
	}

	return TenantKeystores{
		BYOK:      *byokKeystore,
		AllowBYOK: m.flags.IsBYOKAllowed(ctx),
		HYOK:      m.getTenantConfigsHyokKeystore(ctx),
	}, nil
}

// GetKeystoreConfig retrieves the default keystore config
// If the config doesn't exist, it gets the config from the pool and sets it.
// If KeyManagementConfig is not yet provisioned, it lazily calls GrantTrust(MANAGEMENT).
// If CryptoAccessData is missing entries, it syncs via GrantTrust(CRYPTO).
func (m *TenantConfigManager) GetKeystoreConfig(ctx context.Context) (*model.KeystoreConfig, error) {
	keystore, found, err := m.GetStoredDefaultKeystoreConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !found {
		keystore, err = m.initDefaultKeystoreFromPool(ctx)
		if err != nil {
			return nil, err
		}
		return keystore, nil
	}

	if m.certs != nil && m.svcRegistry != nil {
		updated, err := m.ensureKeystoreProvisioned(ctx, keystore)
		if err != nil {
			return nil, err
		}
		if !updated {
			return keystore, nil
		}
		err = m.SetTenantConfig(ctx, keystore)
		if err != nil {
			return nil, err
		}
	}

	return keystore, nil
}

// NeedsDefaultKeystoreProvisioning reports whether the tenant's default keystore
// management role has not yet been fully provisioned. It reads the stored config without
// triggering any lazy provisioning side effects (no GrantTrust calls).
// Returns true when no stored config exists, LocalityID is empty, or AccessData is empty.
func (m *TenantConfigManager) NeedsDefaultKeystoreProvisioning(ctx context.Context) (bool, error) {
	keystore, found, err := m.GetStoredDefaultKeystoreConfig(ctx)
	if err != nil {
		return false, err
	}
	if !found {
		// No stored config yet means the pool assignment hasn't happened — provisioning needed.
		return true, nil
	}
	return keystore.KeyManagementConfig.LocalityID == "" || len(keystore.KeyManagementConfig.AccessData) == 0, nil
}

// GetStoredDefaultKeystoreConfig reads the stored default keystore without
// pool fallback. Reads flat rows first, falling back to the legacy JSON blob.
func (m *TenantConfigManager) GetStoredDefaultKeystoreConfig(ctx context.Context) (*model.KeystoreConfig, bool, error) {
	kc, err := m.GetTenantConfigByType[model.KeystoreConfig](ctx)
	if err != nil {
		return nil, false, errs.Wrap(ErrGetDefaultKeystore, err)
	}
	if kc != nil {
		return kc, true, nil
	}

	return m.getKeystoreConfigFromLegacyBlob(ctx)
}

func (m *TenantConfigManager) initDefaultKeystoreFromPool(ctx context.Context) (*model.KeystoreConfig, error) {
	var keystore *model.KeystoreConfig
	err := m.r.Transaction(ctx, func(ctx context.Context) error {
		var err error
		keystore, err = m.getKeystoreConfigFromPool(ctx)
		if err != nil {
			return err
		}
		return m.SetTenantConfig(ctx, keystore)
	})
	return keystore, err
}

func (m *TenantConfigManager) getTenantConfigsHyokKeystore(ctx context.Context) HYOKKeystore {
	if m.svcRegistry == nil {
		return HYOKKeystore{}
	}

	plugins, err := m.svcRegistry.KeyManagementList()
	if err != nil || len(plugins) == 0 {
		return HYOKKeystore{}
	}

	providers := make([]string, 0)

	for _, plugin := range plugins {
		if pluginHelpers.HasTag(plugin.ServiceInfo().Tags(), string(cmkapi.KeyTypeHYOK)) {
			name := plugin.ServiceInfo().Name()
			if m.flags.IsHYOKAllowed(ctx, name) {
				providers = append(providers, name)
			}
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

func (m *TenantConfigManager) getWorkflowConfigFromLegacyBlob(
	ctx context.Context,
) (*model.WorkflowConfig, bool, error) {
	blob, found, err := m.getLegacyBlob(ctx, constants.WorkflowConfigKey)
	if err != nil || !found {
		return nil, found, err
	}

	var wc model.WorkflowConfig
	if err := json.Unmarshal([]byte(blob), &wc); err != nil {
		return nil, false, errs.Wrap(ErrUnmarshalConfig, err)
	}

	return &wc, true, nil
}

func (m *TenantConfigManager) getKeystoreConfigFromLegacyBlob(
	ctx context.Context,
) (*model.KeystoreConfig, bool, error) {
	blob, found, err := m.getLegacyBlob(ctx, constants.DefaultKeyStore)
	if err != nil || !found {
		return nil, found, err
	}

	keystore := &model.KeystoreConfig{}
	if err := json.Unmarshal([]byte(blob), keystore); err != nil {
		return nil, false, errs.Wrap(ErrUnmarshalConfig, err)
	}

	return keystore, true, nil
}

// getLegacyBlob reads a legacy single-row JSON blob from the jsonb value column.
func (m *TenantConfigManager) getLegacyBlob(
	ctx context.Context,
	key string,
) (string, bool, error) {
	var tc model.LegacyTenantConfig

	ck := repo.NewCompositeKey().
		Where(repo.KeyField, key).
		Where(repo.TypeField, "")
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))

	found, err := m.r.First(ctx, &tc, *query)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return "", false, err
	}
	if !found {
		return "", false, nil
	}

	return tc.Value, true, nil
}

// getDefaultWorkflowConfig returns default workflow config, checking deploymentConfig first,
// then falling back to hard-coded constants
func (m *TenantConfigManager) getDefaultWorkflowConfig(defaultEnabled bool) *model.WorkflowConfig {
	c := &model.WorkflowConfig{
		Enabled:                 defaultEnabled,
		MinimumApprovals:        constants.DefaultMinimumApprovalCount,
		MaxApprovals:            constants.MaxMinimumApprovals,
		RetentionPeriodDays:     constants.DefaultRetentionPeriodDays,
		MinRetentionPeriodDays:  constants.MinRetentionPeriodDays,
		MaxRetentionPeriodDays:  constants.MaxRetentionPeriodDays,
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
// to override any default values. Values are clamped to hard limits.
func (m *TenantConfigManager) applyDeploymentConfigOverrides(config *model.WorkflowConfig) {
	if m.cfg.Workflow.DefaultMinimumApprovals > 0 {
		v := min(m.cfg.Workflow.DefaultMinimumApprovals, constants.MaxMinimumApprovals)
		config.MinimumApprovals = v
	}
	if m.cfg.Workflow.DefaultRetentionPeriodDays > 0 {
		v := max(m.cfg.Workflow.DefaultRetentionPeriodDays, constants.MinRetentionPeriodDays)
		config.RetentionPeriodDays = min(v, constants.MaxRetentionPeriodDays)
	}
	if m.cfg.Workflow.DefaultExpiryPeriodDays > 0 {
		config.DefaultExpiryPeriodDays = m.cfg.Workflow.DefaultExpiryPeriodDays
	}
	if m.cfg.Workflow.DefaultMaxExpiryPeriodDays > 0 {
		v := min(m.cfg.Workflow.DefaultMaxExpiryPeriodDays, constants.DefaultMaxExpiryPeriodDays)
		config.MaxExpiryPeriodDays = v
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

// ensureKeystoreProvisioned lazily provisions KeyManagementConfig and syncs CryptoAccessData.
// Returns true if ksConfig was mutated and should be persisted.
func (m *TenantConfigManager) ensureKeystoreProvisioned(
	ctx context.Context,
	ksConfig *model.KeystoreConfig,
) (bool, error) {
	updated := false

	if ksConfig.KeyManagementConfig.LocalityID == "" {
		if err := m.provisionKeyManagementRole(ctx, ksConfig); err != nil {
			return false, err
		}
		updated = true
	}

	cryptoUpdated, err := m.syncCryptoAccessData(ctx, ksConfig)
	if err != nil {
		return false, err
	}

	return updated || cryptoUpdated, nil
}

// provisionKeyManagementRole calls GrantTrust(MANAGEMENT) using the role-management cert
// and stores the result in ksConfig.KeyManagementConfig.
func (m *TenantConfigManager) provisionKeyManagementRole(
	ctx context.Context,
	ksConfig *model.KeystoreConfig,
) error {
	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return errs.Wrap(ErrGetTenantFromCtx, err)
	}

	client, err := m.getKeystoreManagementClient()
	if err != nil {
		return err
	}

	configMap, err := m.buildRoleManagementConfigMap(ctx, ksConfig)
	if err != nil {
		return err
	}

	keyMgmtCN := m.cfg.Certificates.DefaultTenantCertPrefix + defaultKeystoreCertInfix + tenantID

	resp, err := client.GrantTrust(ctx, &keystoremanagement.GrantTrustRequest{
		Config:  common.KeystoreConfig{Values: configMap},
		Subject: keyMgmtCN,
		Region:  ksConfig.RoleManagementConfig.LocalityID,
		Type:    keystoremanagement.TrustTypeManagement,
	})
	if err != nil {
		return errs.Wrap(ErrGrantTrustFailed, err)
	}

	ksConfig.KeyManagementConfig = model.ManagementConfig{
		LocalityID: ksConfig.RoleManagementConfig.LocalityID,
		CommonName: keyMgmtCN,
		AccessData: resp.AccessData.Values,
	}

	return nil
}

// syncCryptoAccessData ensures all configured crypto certs are trusted.
// Returns true if ksConfig was mutated.
func (m *TenantConfigManager) syncCryptoAccessData(
	ctx context.Context,
	ksConfig *model.KeystoreConfig,
) (bool, error) {
	cryptoCerts, err := m.certs.getCryptoCertificates(ctx)
	if err != nil {
		return false, err
	}

	if len(cryptoCerts) == 0 {
		return false, nil
	}

	if ksConfig.CryptoAccessData == nil {
		ksConfig.CryptoAccessData = make(map[string]model.CryptoConfig)
	}

	updated := false
	for _, cert := range cryptoCerts {
		certUpdated, err := m.syncCert(ctx, cert, ksConfig)
		if err != nil {
			return false, err
		}
		updated = updated || certUpdated
	}

	return updated, nil
}

func (m *TenantConfigManager) syncCert(
	ctx context.Context,
	cert *model.ClientCertificate,
	ksConfig *model.KeystoreConfig,
) (bool, error) {
	if _, exists := ksConfig.CryptoAccessData[cert.Name]; exists {
		return false, nil
	}

	accessData, err := m.grantCryptoRoleTrust(ctx, cert.Subject.String(), cert.Name, ksConfig)
	if err != nil {
		return false, err
	}

	ksConfig.CryptoAccessData[cert.Name] = model.CryptoConfig{
		Subject:    cert.Subject.String(),
		AccessData: accessData.Values,
	}

	return true, nil
}

func (m *TenantConfigManager) grantCryptoRoleTrust(
	ctx context.Context,
	subject, region string,
	ksConfig *model.KeystoreConfig,
) (*common.KeystoreConfig, error) {
	client, err := m.getKeystoreManagementClient()
	if err != nil {
		return nil, err
	}

	configMap, err := m.buildRoleManagementConfigMap(ctx, ksConfig)
	if err != nil {
		return nil, err
	}

	resp, err := client.GrantTrust(ctx, &keystoremanagement.GrantTrustRequest{
		Config:  common.KeystoreConfig{Values: configMap},
		Subject: subject,
		Region:  region,
		Type:    keystoremanagement.TrustTypeCrypto,
	})
	if err != nil {
		return nil, errs.Wrap(ErrGrantTrustFailed, err)
	}

	return &common.KeystoreConfig{Values: resp.AccessData.Values}, nil
}

// buildRoleManagementConfigMap builds the config map for GrantTrust calls
// by combining the role-management cert with the role-management access data from ksConfig.
func (m *TenantConfigManager) buildRoleManagementConfigMap(
	ctx context.Context,
	ksConfig *model.KeystoreConfig,
) (map[string]any, error) {
	cert, err := m.getRoleManagementCert(ctx, ksConfig)
	if err != nil {
		return nil, err
	}

	configMap := map[string]any{
		"authType":   constants.AuthTypeCertificate,
		"clientCert": cert.CertPEM,
		"privateKey": cert.PrivateKeyPEM,
	}
	maps.Copy(configMap, ksConfig.RoleManagementConfig.AccessData)

	return configMap, nil
}

func (m *TenantConfigManager) getRoleManagementCert(
	ctx context.Context,
	ksConfig *model.KeystoreConfig,
) (*model.Certificate, error) {
	return m.certs.getDefaultKeystoreClientCert(
		ctx,
		ksConfig.RoleManagementConfig.LocalityID,
		ksConfig.RoleManagementConfig.CommonName,
		model.CertificatePurposeRoleManagement,
	)
}

func (m *TenantConfigManager) getKeystoreManagementClient() (keystoremanagement.KeystoreManagement, error) {
	clients, err := m.svcRegistry.KeystoreManagements()
	if err != nil {
		return nil, errs.Wrap(ErrGetDefaultKeystore, err)
	}

	plugins, err := m.svcRegistry.KeyManagementList()
	if err != nil || len(plugins) == 0 {
		return nil, errs.Wrapf(ErrGetDefaultKeystore, "no keystore plugins found")
	}

	for _, plugin := range plugins {
		if pluginHelpers.HasTag(plugin.ServiceInfo().Tags(), constants.DefaultKeyStore) {
			client, ok := clients[plugin.ServiceInfo().Name()]
			if ok {
				return client, nil
			}
		}
	}

	return nil, errs.Wrapf(ErrGetDefaultKeystore, "no default keystore management client found")
}

// regionsByTechnicalName indexes a slice of Regions by TechnicalName.
func regionsByTechnicalName(regions []config.Region) map[string]config.Region {
	m := make(map[string]config.Region, len(regions))
	for _, r := range regions {
		m[r.TechnicalName] = r
	}
	return m
}
