package servicewrapper

import (
	"time"

	"github.com/openkcm/plugin-sdk/api"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	"github.com/openkcm/cmk/internal/pluginregistry/service/wrapper/identity_management"
	"github.com/openkcm/cmk/utils/cache"
)

type identityManagementRepository struct {
	identity_management.Repository
}

func (repo *identityManagementRepository) Binder() any {
	return repo.SetIdentityManagement
}

func (repo *identityManagementRepository) Constraints() api.Constraints {
	return api.MaybeOne()
}

func (repo *identityManagementRepository) Versions() []api.Version {
	return []api.Version{identityManagementV1{}}
}

type identityManagementV1 struct{}

func (identityManagementV1) New() api.Facade {
	return &identity_management.V1{
		UserCache: cache.NewTTLCache[string, identitymanagement.User](cache.TTLConfig{
			ItemTTL: 1 * time.Hour,
			GCConfig: cache.TTLGC{
				Enabled:  true,
				Interval: 6 * time.Hour,
			},
		}),
	}
}
func (identityManagementV1) Deprecated() bool { return false }
