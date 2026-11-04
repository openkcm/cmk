package tenantconfigs

import (
	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/model"
)

func LimitsToAPI(limits model.TenantLimits) *cmkapi.TenantLimits {
	return &cmkapi.TenantLimits{
		Systems:           limits.Systems,
		Keys:              limits.Keys,
		KeyConfigurations: limits.KeyConfigs,
	}
}
