package keystore_management

import (
	"log/slog"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keystoremanagement"
)

type Repository struct {
	Instances map[string]keystoremanagement.KeystoreManagement
}

func (repo *Repository) KeystoreManagements() (map[string]keystoremanagement.KeystoreManagement, bool) {
	return repo.Instances, len(repo.Instances) > 0
}

func (repo *Repository) KeystoreManagementList() ([]keystoremanagement.KeystoreManagement, bool) {
	if len(repo.Instances) == 0 {
		return nil, false
	}

	list := make([]keystoremanagement.KeystoreManagement, 0, len(repo.Instances))
	for _, management := range repo.Instances {
		list = append(list, management)
	}
	return list, true
}

func (repo *Repository) AddKeystoreManagement(instance keystoremanagement.KeystoreManagement) {
	if repo.Instances == nil {
		repo.Instances = make(map[string]keystoremanagement.KeystoreManagement)
	}

	info := instance.ServiceInfo()
	if info == nil {
		slog.Error("FATAL:Service info of KeystoreManagement is required!")
		return
	}

	repo.Instances[info.Name()] = instance
}

func (repo *Repository) Clear() {
	repo.Instances = nil
}
