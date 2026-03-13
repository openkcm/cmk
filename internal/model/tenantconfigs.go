package model

import (
	"context"
	"encoding/json"

	"github.com/openkcm/cmk/internal/authz"
)

// TenantConfig represents a key in the database.
type TenantConfig struct {
	Key   string          `gorm:"type:varchar(255);primaryKey"`
	Value json.RawMessage `gorm:"type:jsonb;not null"`
}

// TableResourceType return the authz resource type
func (m TenantConfig) TableResourceType() authz.RepoResourceTypeName {
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
	authzHandler *authz.Handler[authz.RepoResourceTypeName, authz.RepoAction],
	action authz.RepoAction) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
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
