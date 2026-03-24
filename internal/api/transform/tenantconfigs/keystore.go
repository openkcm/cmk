package tenantconfigs

import (
	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/utils/ptr"
	"github.com/openkcm/cmk/utils/sanitise"
)

// ToAPI transforms a system model to an API system.
func ToAPI(keystore manager.TenantKeystores) (*cmkapi.TenantKeystore, error) {
	err := sanitise.Sanitize(&keystore)
	if err != nil {
		return nil, err
	}

	supportedRegions := make([]cmkapi.SupportedRegion, len(keystore.BYOK.SupportedRegions))
	for i, r := range keystore.BYOK.SupportedRegions {
		supportedRegions[i] = cmkapi.SupportedRegion{
			Name:          &r.Name,
			TechnicalName: &r.TechnicalName,
		}
	}

	apiTenant := &cmkapi.TenantKeystore{
		Byok: &cmkapi.BYOKKeystore{
			Allow:            ptr.PointTo(false),
			SupportedRegions: &supportedRegions,
		},
		Hyok: cmkapi.HYOKKeystore{
			Providers: &keystore.HYOK.Provider,
			Allow:     &keystore.HYOK.Allow,
		},
	}

	return apiTenant, nil
}
