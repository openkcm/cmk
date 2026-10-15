package tenantconfigs

import (
	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/model"
)

func SystemGroupsConfigToAPI(sg *model.SystemGroupConfig) *cmkapi.TenantSystemGroupsConfig {
	return &cmkapi.TenantSystemGroupsConfig{
		Enabled:      &sg.Enabled,
		WarnCoverage: &sg.WarnCoverage,
	}
}
