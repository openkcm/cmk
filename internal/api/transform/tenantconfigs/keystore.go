package tenantconfigs

import (
	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/manager"
)

// ToAPI transforms a system model to an API system.
func ToAPI(keystore manager.TenantKeystores) *cmkapi.TenantKeystore {
	apiTenant := &cmkapi.TenantKeystore{
		//nolint:godox
		Default: nil, // TODO: AS PER KMS20-2740 this is disabled. To be enabled on KMS20-2742
		Hyok: cmkapi.HYOKKeystore{
			Providers: &keystore.HYOK.Provider,
			Allow:     &keystore.HYOK.Allow,
		},
	}

	return apiTenant
}
