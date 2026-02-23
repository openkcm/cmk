package servicewrapper

import (
	"github.com/openkcm/plugin-sdk/api"

	"github.com/openkcm/cmk/internal/pluginregistry/service/wrapper/keystore_management"
)

type keystoreManagementRepository struct {
	keystore_management.Repository
}

func (repo *keystoreManagementRepository) Binder() any {
	return repo.AddKeystoreManagement
}

func (repo *keystoreManagementRepository) Constraints() api.Constraints {
	return api.ZeroOrMore()
}

func (repo *keystoreManagementRepository) Versions() []api.Version {
	return []api.Version{keystoreManagementV1{}}
}

type keystoreManagementV1 struct{}

func (keystoreManagementV1) New() api.Facade  { return new(keystore_management.V1) }
func (keystoreManagementV1) Deprecated() bool { return false }
