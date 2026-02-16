package servicewrapper

import (
	"github.com/openkcm/plugin-sdk/api"

	"github.com/openkcm/cmk/internal/pluginregistry/service/wrapper/key_management"
)

type keyManagementRepository struct {
	key_management.Repository
}

func (repo *keyManagementRepository) Binder() any {
	return repo.AddKeyManagement
}

func (repo *keyManagementRepository) Constraints() api.Constraints {
	return api.ExactlyOne()
}

func (repo *keyManagementRepository) Versions() []api.Version {
	return []api.Version{keyManagementV1{}}
}

type keyManagementV1 struct{}

func (keyManagementV1) New() api.Facade {
	return new(key_management.V1)
}
func (keyManagementV1) Deprecated() bool { return false }
