package tenantconfigs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/tenantconfigs"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
)

func TestToAPI(t *testing.T) {
	providers := []string{"AWS"}
	expected := cmkapi.TenantKeystore{
		Default: nil,
		Hyok: cmkapi.HYOKKeystore{
			Providers: &providers,
			Allow:     ptr.PointTo(true),
		},
	}

	keyStore := manager.TenantKeystores{
		Default: model.DefaultKeystore{},
		HYOK: manager.HYOKKeystore{
			Provider: providers,
			Allow:    true,
		},
	}

	res, err := tenantconfigs.ToAPI(keyStore)

	assert.NoError(t, err)
	assert.Equal(t, expected, *res)
}
