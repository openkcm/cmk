package tenantconfigs

import cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"

func LimitsToAPI(systems, keys, keyConfigs int) *cmkapi.TenantLimits {
	return &cmkapi.TenantLimits{
		Systems:           &systems,
		Keys:              &keys,
		KeyConfigurations: &keyConfigs,
	}
}
