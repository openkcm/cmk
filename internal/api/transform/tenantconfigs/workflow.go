package tenantconfigs

import (
	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/model"
)

// WorkflowConfigToAPI transforms a model.WorkflowConfig to an API TenantWorkflowConfiguration.
func WorkflowConfigToAPI(config *model.WorkflowConfig) *cmkapi.TenantWorkflowConfiguration {
	if config == nil {
		return nil
	}

	return &cmkapi.TenantWorkflowConfiguration{
		Enabled:                 new(config.Enabled),
		MinimumApprovals:        new(config.MinimumApprovals),
		RetentionPeriodDays:     new(config.RetentionPeriodDays),
		DefaultExpiryPeriodDays: new(config.DefaultExpiryPeriodDays),
		MaxExpiryPeriodDays:     new(config.MaxExpiryPeriodDays),
	}
}
