package tenantconfigs

import (
	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
)

// WorkflowConfigToAPI transforms a model.WorkflowConfig to an API TenantWorkflowConfiguration.
func WorkflowConfigToAPI(config *model.WorkflowConfig) *cmkapi.TenantWorkflowConfiguration {
	if config == nil {
		return nil
	}

	return &cmkapi.TenantWorkflowConfiguration{
		Enabled:                 ptr.PointTo(config.Enabled),
		MinimumApprovals:        ptr.PointTo(config.MinimumApprovals),
		RetentionPeriodDays:     ptr.PointTo(config.RetentionPeriodDays),
		DefaultExpiryPeriodDays: ptr.PointTo(config.DefaultExpiryPeriodDays),
		MaxExpiryPeriodDays:     ptr.PointTo(config.MaxExpiryPeriodDays),
	}
}
