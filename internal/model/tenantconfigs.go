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
// Key + Type form the composite primary key (Type groups related entries, e.g. "workflow").
type TenantConfig struct {
	Key   string     `gorm:"type:varchar(255);primaryKey"`
	Value string     `gorm:"type:text;not null"`
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
	Enabled                       bool `flatten:"enabled"`
	RequiredApprovals             int  `flatten:"required_approvals"`
	MinAllowedApprovals           int  `flatten:"min_allowed_approvals"`
	MaxAllowedApprovals           int  `flatten:"max_allowed_approvals"`
	RetentionPeriodDays           int  `flatten:"retention_period_days"`
	MinAllowedRetentionPeriodDays int  `flatten:"min_allowed_retention_period_days"`
	MaxAllowedRetentionPeriodDays int  `flatten:"max_allowed_retention_period_days"`
	ExpiryPeriodDays              int  `flatten:"expiry_period_days"`
	MaxAllowedExpiryPeriodDays    int  `flatten:"max_allowed_expiry_period_days"`
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
