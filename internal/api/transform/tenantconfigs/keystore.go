package tenantconfigs

import (
	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/utils/sanitise"
)

// ToAPI transforms a system model to an API system.
func ToAPI(keystore manager.TenantKeystores) (*cmkapi.TenantKeystore, error) {
	err := sanitise.Stringlikes(&keystore)
	if err != nil {
		return nil, err
	}

	apiTenant := &cmkapi.TenantKeystore{
		Default: nil,
		Hyok: cmkapi.HYOKKeystore{
			Providers: &keystore.HYOK.Provider,
			Allow:     &keystore.HYOK.Allow,
		},
	}

	return apiTenant, nil
}
