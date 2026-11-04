package tenantconfigs

import (
	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/model"
)

// WorkflowConfigToAPI transforms a model.WorkflowConfig to an API TenantWorkflowConfiguration.
func WorkflowConfigToAPI(config *model.WorkflowConfig) *cmkapi.TenantWorkflowConfiguration {
	if config == nil {
		return nil
	}

	return &cmkapi.TenantWorkflowConfiguration{
		Enabled:                       new(config.Enabled),
		RequiredApprovals:             new(config.RequiredApprovals),
		MinAllowedApprovals:           new(config.MinAllowedApprovals),
		MaxAllowedApprovals:           new(config.MaxAllowedApprovals),
		RetentionPeriodDays:           new(config.RetentionPeriodDays),
		MinAllowedRetentionPeriodDays: new(config.MinAllowedRetentionPeriodDays),
		MaxAllowedRetentionPeriodDays: new(config.MaxAllowedRetentionPeriodDays),
		ExpiryPeriodDays:              new(config.ExpiryPeriodDays),
		MaxAllowedExpiryPeriodDays:    new(config.MaxAllowedExpiryPeriodDays),
	}
}
