package manager

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"regexp"
	"strconv"
	"strings"

	tenantpb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	"github.com/openkcm/common-sdk/pkg/commoncfg"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	servicewrapper "github.com/openkcm/cmk/internal/pluginregistry/service/wrapper"
	"github.com/openkcm/cmk/internal/repo"
	pluginHelpers "github.com/openkcm/cmk/utils/plugins"
)

const (
	DefaultCertName = "hyok-default"

	// Since the workflow expiry must be less than the retention minus a day
	minimumRetentionPeriodDays = 2
)

var (
	ErrGetDefaultCerts = errors.New("failed to get default certificates")
	ErrDecodingCert    = errors.New("failed to decode certificate")
)

type TenantConfigManager struct {
	repo         repo.Repo
	certs        *CertificateManager
	svcRegistry  *cmkpluginregistry.Registry
	keystorePool *Pool
	cfg          *config.Config
}

func NewTenantConfigManager(
	repo repo.Repo,
	certManager *CertificateManager,
	svcRegistry *cmkpluginregistry.Registry,
	deploymentConfig *config.Config,
) *TenantConfigManager {
	return &TenantConfigManager{
		repo:         repo,
		certs:        certManager,
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
)

type HYOKKeystore struct {
	Provider []string `json:"provider"`
	Allow    bool
}

type TenantKeystores struct {
	Default model.KeystoreConfig
	HYOK    HYOKKeystore
}

func (m *TenantConfigManager) GetWorkflowConfig(ctx context.Context) (*model.WorkflowConfig, error) {
	var tenantConfig model.TenantConfig

	ck := repo.NewCompositeKey().Where(repo.KeyField, constants.WorkflowConfigKey)
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(ck),
	)

	found, err := m.repo.First(ctx, &tenantConfig, *query)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, errs.Wrap(ErrGetWorkflowConfig, err)
	}

	if !found {
		return m.SetWorkflowConfig(ctx, nil)
	}

	// Convert TenantConfig to WorkflowConfig
	workflowConfig, err := m.convertToWorkflowConfig(&tenantConfig)
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

		workflowConfig = m.getDefaultWorkflowConfig(defaultEnabled)
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

func (m *TenantConfigManager) GetTenantsKeystores() (TenantKeystores, error) {
	defaultKeystore := model.KeystoreConfig{}

	return TenantKeystores{
		Default: defaultKeystore,
		HYOK:    m.getTenantConfigsHyokKeystore(),
	}, nil
}

// GetDefaultKeystoreConfig retrieves the default keystore config
// If the config doesn't exist, it gets the config from the pool and sets it
func (m *TenantConfigManager) GetDefaultKeystoreConfig(ctx context.Context) (*model.KeystoreConfig, error) {
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
		var keystore *model.KeystoreConfig

		err = m.repo.Transaction(ctx, func(ctx context.Context) error {
			keystore, err = m.getKeystoreConfigFromPool(ctx)
			if err != nil {
				return err
			}

			err = m.setDefaultKeystore(ctx, keystore)
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

	keystore := &model.KeystoreConfig{}

	err = json.Unmarshal(config.Value, keystore)
	if err != nil {
		return nil, errs.Wrap(ErrUnmarshalConfig, err)
	}

	return keystore, nil
}

// ClientCertificate represents the client certificates
type ClientCertificate struct {
	Name    string
	RootCA  string
	Subject string
}

// GetClientCertificates retrieves the client certificates
func (m *TenantConfigManager) GetClientCertificates(ctx context.Context) (
	map[model.CertificatePurpose][]*ClientCertificate, error,
) {
	certConfig := m.certs.cfg

	tenantDefaultCert, err := m.certs.getDefaultHYOKClientCert(ctx)
	if err != nil {
		return nil, errs.Wrap(ErrGetDefaultCerts, err)
	}

	defaultCerts := []*model.Certificate{tenantDefaultCert}

	clientCerts := make(map[model.CertificatePurpose][]*ClientCertificate)
	clientCerts[model.CertificatePurposeTenantDefault] = make([]*ClientCertificate, len(defaultCerts))

	for i, certificate := range defaultCerts {
		configCert, err := m.transformTenantDefaultCertificate(ctx, certificate.CertPEM,
			certConfig.Certificates.RootCertURL, ErrGetDefaultCerts)
		if err != nil {
			return nil, err
		}

		clientCerts[model.CertificatePurposeTenantDefault][i] = configCert
	}

	cryptoCerts, err := m.getCryptoCertificates()
	clientCerts[model.CertificatePurposeCrypto] = cryptoCerts

	if err != nil {
		return nil, err
	}

	return clientCerts, nil
}

func (m *TenantConfigManager) transformTenantDefaultCertificate(_ context.Context,
	certRaw, rootCertURL string, errParent error,
) (*ClientCertificate, error) {
	block, _ := pem.Decode([]byte(certRaw))
	if block == nil {
		return nil, errs.Wrap(errParent, ErrDecodingCert)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errs.Wrap(errParent, err)
	}

	subject := formatSubjectWithSlashSeparatedOUs(cert.Subject)

	return &ClientCertificate{
		Name:    DefaultCertName,
		RootCA:  rootCertURL,
		Subject: subject,
	}, nil
}

// formatSubjectWithSlashSeparatedOUs transforms the standard X.509 subject string
// to combine multiple OUs with / separator instead of +
func formatSubjectWithSlashSeparatedOUs(subject pkix.Name) string {
	if len(subject.OrganizationalUnit) <= 1 {
		return subject.String() // Use standard format if 0 or 1 OU
	}

	// Get standard format
	standardSubject := subject.String()

	// Replace OU=X+OU=Y+OU=Z with OU=X/Y/Z
	combinedOU := "OU=" + strings.Join(subject.OrganizationalUnit, "/")

	// Build regex to match multiple OU entries
	ouPattern := `OU=[^,+]+((\+OU=[^,+]+)+)`
	re := regexp.MustCompile(ouPattern)

	return re.ReplaceAllString(standardSubject, combinedOU)
}

// getCryptoCertificates retrieves crypto certificates from config
func (m *TenantConfigManager) getCryptoCertificates() ([]*ClientCertificate, error) {
	bytes, err := commoncfg.LoadValueFromSourceRef(m.cfg.CryptoLayer.CertX509Trusts)
	if err != nil {
		return nil, errs.Wrap(ErrLoadCryptoCerts, err)
	}

	var cryptoCerts map[string]ClientCertificate

	err = json.Unmarshal(bytes, &cryptoCerts)
	if err != nil {
		return nil, errs.Wrap(ErrUnmarshalCryptoCerts, err)
	}

	return m.certMapToSlice(cryptoCerts), nil
}

func (m *TenantConfigManager) certMapToSlice(certs map[string]ClientCertificate) []*ClientCertificate {
	l := make([]*ClientCertificate, 0, len(certs))
	for k, v := range certs {
		l = append(l, &ClientCertificate{
			Name:    k,
			Subject: v.Subject,
			RootCA:  v.RootCA,
		})
	}

	return l
}

// SetDefaultKeystore stores the default keystore config
func (m *TenantConfigManager) setDefaultKeystore(ctx context.Context, keystore *model.KeystoreConfig) error {
	ksBytes, err := json.Marshal(keystore)
	if err != nil {
		return errs.Wrap(ErrMarshalConfig, err)
	}

	conf := &model.TenantConfig{
		Key:   constants.DefaultKeyStore,
		Value: ksBytes,
	}

	err = m.repo.Set(ctx, conf)
	if err != nil {
		return errs.Wrap(ErrSetDefaultKeystore, err)
	}

	return nil
}

func (m *TenantConfigManager) getTenantConfigsHyokKeystore() HYOKKeystore {
	if m.svcRegistry == nil {
		return HYOKKeystore{}
	}

	plugins := m.svcRegistry.LookupByType(servicewrapper.KeyManagementType)
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

// convertToWorkflowConfig converts TenantConfig to WorkflowConfig
func (m *TenantConfigManager) convertToWorkflowConfig(config *model.TenantConfig) (*model.WorkflowConfig, error) {
	var workflowConfig model.WorkflowConfig

	err := json.Unmarshal(config.Value, &workflowConfig)
	if err != nil {
		return nil, err
	}

	return &workflowConfig, nil
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
