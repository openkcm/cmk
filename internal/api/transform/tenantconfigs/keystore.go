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
	// Hardcoded allowManaged=true and allowBYOK=false for now
	allowManaged := true
	allowBYOK := false
	apiTenant := &cmkapi.TenantKeystore{
		Default: &cmkapi.DefaultKeystore{
			AllowManaged: &allowManaged,
			AllowBYOK:    &allowBYOK,
		},
		Hyok: cmkapi.HYOKKeystore{
			Providers: &keystore.HYOK.Provider,
			Allow:     &keystore.HYOK.Allow,
		},
	}

	return apiTenant, nil
}
