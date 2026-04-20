package tenantconfigs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/tenantconfigs"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
)

func TestToAPI(t *testing.T) {
	providers := []string{"AWS"}
	regionName := "Europe"
	regionTechnicalName := "eu-west-1"
	supportedRegions := []cmkapi.SupportedRegion{
		{Name: &regionName, TechnicalName: &regionTechnicalName},
	}
	testCases := []struct {
		name      string
		allowBYOK bool
	}{
		{
			name:      "maps allowBYOK as false",
			allowBYOK: false,
		},
		{
			name:      "maps allowBYOK as true",
			allowBYOK: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expected := cmkapi.TenantKeystore{
				Byok: &cmkapi.BYOKKeystore{
					Allow:            ptr.PointTo(tc.allowBYOK),
					SupportedRegions: &supportedRegions,
				},
				Hyok: cmkapi.HYOKKeystore{
					Providers: &providers,
					Allow:     ptr.PointTo(true),
				},
			}

			keyStore := manager.TenantKeystores{
				BYOK: model.KeystoreConfig{
					SupportedRegions: []config.Region{
						{Name: regionName, TechnicalName: regionTechnicalName},
					},
				},
				AllowBYOK: tc.allowBYOK,
				HYOK: manager.HYOKKeystore{
					Provider: providers,
					Allow:    true,
				},
			}

			res, err := tenantconfigs.ToAPI(keyStore)

			assert.NoError(t, err)
			assert.Equal(t, expected, *res)
		})
	}
}
