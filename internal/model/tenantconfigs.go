package model

import "encoding/json"

// TenantConfig represents a key in the database.
type TenantConfig struct {
	Key   string          `gorm:"type:varchar(255);primaryKey"`
	Value json.RawMessage `gorm:"type:jsonb;not null"`
}

// TableName returns the table name for Key
func (TenantConfig) TableName() string {
	return "tenant_configs"
}

func (TenantConfig) IsSharedModel() bool {
	return false
}

type WorkflowConfig struct {
	// Enabled determines if workflows are enabled in controllers
	Enabled bool

	// MinimumApprovals is the minimum number of approvals required for a workflow
	MinimumApprovals int

	// RetentionPeriodDays is the number of days to retain workflow data
	RetentionPeriodDays int

	// DefaultExpiryPeriodDays is the default number of days after which pending workflows will expire
	DefaultExpiryPeriodDays int

	// MaxExpiryPeriodDays is the maximum settable value for the expiry period
	MaxExpiryPeriodDays int
}
