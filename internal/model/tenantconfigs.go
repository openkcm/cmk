package model

import (
	"context"

	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/config"
)

type ConfigType string

const (
	TenantConfigTypeWorkflow        ConfigType = "workflow"
	TenantConfigTypeDefaultKeystore ConfigType = "default_keystore"
	TenantConfigTypeLimits          ConfigType = "limits"
	TenantConfigTypeSystemGrouping  ConfigType = "system_grouping"
)

// TenantConfig represents a flat config row in the database.
// Key + Type form the composite key (Type groups related entries, e.g. "workflow").
// Flat rows store their value in value_text; legacy jsonb blobs are read via
// LegacyTenantConfig.
type TenantConfig struct {
	Key   string     `gorm:"type:varchar(255);primaryKey"`
	Value string     `gorm:"column:value_text;type:text"`
	Type  ConfigType `gorm:"type:varchar(255);primaryKey;default:''"`
}

type TenantConfiguration interface {
	Flattenable
	Type() ConfigType
}

// TableResourceType return the authz resource type
func (m TenantConfig) TableResourceType() authz.RepoResourceType {
	return authz.RepoResourceTypeTenantconfig
}

// TableName returns the table name for Key
func (m TenantConfig) TableName() string {
	return string(m.TableResourceType())
}

func (TenantConfig) IsSharedModel() bool {
	return false
}

func (m TenantConfig) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceType, authz.RepoAction],
	action authz.RepoAction,
) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}

// LegacyTenantConfig reads the legacy jsonb value column, serving blob reads
// during the flatten rollout until the cleanup release drops that column.
type LegacyTenantConfig struct {
	Key   string `gorm:"type:varchar(255);primaryKey"`
	Value string `gorm:"column:value;type:jsonb"`
	Type  string `gorm:"type:varchar(255);primaryKey;default:''"`
}

func (LegacyTenantConfig) TableResourceType() authz.RepoResourceType {
	return authz.RepoResourceTypeTenantconfig
}

func (m LegacyTenantConfig) TableName() string {
	return string(m.TableResourceType())
}

func (LegacyTenantConfig) IsSharedModel() bool {
	return false
}

func (m LegacyTenantConfig) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceType, authz.RepoAction], action authz.RepoAction,
) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}

//nolint:lll
type KeystoreConfig struct {
	RoleManagementConfig ManagementConfig         `yaml:"roleManagementConfig" json:"roleManagementConfig" flatten:"role_mgmt"`
	KeyManagementConfig  ManagementConfig         `yaml:"keyManagementConfig" json:"keyManagementConfig" flatten:"key_mgmt"`
	CryptoAccessData     map[string]CryptoConfig  `yaml:"cryptoAccessData" json:"cryptoAccessData" flatten:"crypto"`
	SupportedRegions     map[string]config.Region `yaml:"supportedRegions" json:"supportedRegions" flatten:"supported_region"`
}

//nolint:tagliatelle
type ManagementConfig struct {
	LocalityID string             `yaml:"localityId" json:"localityId" flatten:"locality_id"`
	CommonName string             `yaml:"commonName" json:"commonName" flatten:"common_name"`
	AccessData KeystoreAccessData `yaml:"accessData" json:"accessData" flatten:"access_data"`
}

type CryptoConfig struct {
	Subject    string             `yaml:"subject" json:"subject" flatten:"subject"`
	AccessData KeystoreAccessData `yaml:"accessData" json:"accessData" flatten:"access_data"`
}

type KeystoreAccessData map[string]any

func (c KeystoreConfig) FlattenKeyRoot() string {
	return "keystore_config"
}

func (c KeystoreConfig) Type() ConfigType {
	return TenantConfigTypeDefaultKeystore
}

type WorkflowConfig struct {
	Enabled                 bool `flatten:"enabled"`
	MinimumApprovals        int  `flatten:"minimum_approvals"`
	MaxApprovals            int  `flatten:"max_approvals"`
	RetentionPeriodDays     int  `flatten:"retention_period_days"`
	MinRetentionPeriodDays  int  `flatten:"min_retention_period_days"`
	MaxRetentionPeriodDays  int  `flatten:"max_retention_period_days"`
	DefaultExpiryPeriodDays int  `flatten:"default_expiry_period_days"`
	MaxExpiryPeriodDays     int  `flatten:"max_expiry_period_days"`
}

func (c WorkflowConfig) FlattenKeyRoot() string {
	return ""
}

func (c WorkflowConfig) Type() ConfigType {
	return TenantConfigTypeWorkflow
}

type SystemGroupConfig struct {
	Enabled      bool `flatten:"enabled"`
	WarnCoverage bool `flatten:"warn_coverage"`
}

func (c SystemGroupConfig) FlattenKeyRoot() string {
	return ""
}

func (c SystemGroupConfig) Type() ConfigType {
	return TenantConfigTypeSystemGrouping
}

type TenantLimits struct {
	Systems    *int `flatten:"system"`
	Keys       *int `flatten:"keys"`
	KeyConfigs *int `flatten:"key_configs"`
}

func (c TenantLimits) FlattenKeyRoot() string {
	return ""
}

func (c TenantLimits) Type() ConfigType {
	return TenantConfigTypeLimits
}
