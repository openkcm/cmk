package servicewrapper

import (
	"github.com/openkcm/plugin-sdk/api"

	"github.com/openkcm/cmk/internal/pluginregistry/service/wrapper/identity_management"
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

func (identityManagementV1) New() api.Facade  { return new(identity_management.V1) }
func (identityManagementV1) Deprecated() bool { return false }
