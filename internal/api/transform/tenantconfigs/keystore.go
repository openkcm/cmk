package tenantconfigs

import (
	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/utils/sanitise"
)

// ToAPI transforms a system model to an API system.
func ToAPI(keystore manager.TenantKeystores) (*cmkapi.TenantKeystore, error) {
	err := sanitise.Sanitize(&keystore)
	if err != nil {
		return nil, err
	}

	apiTenant := &cmkapi.TenantKeystore{
		//nolint:godox
		Default: nil, // TODO: AS PER KMS20-2740 this is disabled. To be enabled on KMS20-2742
		Hyok: cmkapi.HYOKKeystore{
			Providers: &keystore.HYOK.Provider,
			Allow:     &keystore.HYOK.Allow,
		},
	}

	return apiTenant, nil
}
