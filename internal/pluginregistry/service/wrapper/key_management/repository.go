package key_management

import (
	"log/slog"

	keymanagementapi "github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
)

type Repository struct {
	Instances map[string]keymanagementapi.KeyManagement
}

func (repo *Repository) KeyManagements() (map[string]keymanagementapi.KeyManagement, bool) {
	return repo.Instances, len(repo.Instances) > 0
}

func (repo *Repository) KeyManagementList() ([]keymanagementapi.KeyManagement, bool) {
	if len(repo.Instances) == 0 {
		return nil, false
	}

	list := make([]keymanagementapi.KeyManagement, 0, len(repo.Instances))
	for _, manager := range repo.Instances {
		list = append(list, manager)
	}
	return list, true
}

func (repo *Repository) AddKeyManagement(instance keymanagementapi.KeyManagement) {
	if repo.Instances == nil {
		repo.Instances = make(map[string]keymanagementapi.KeyManagement)
	}

	info := instance.ServiceInfo()
	if info == nil {
		slog.Error("FATAL:Service info of KeyManagement is required!")
		return
	}

	repo.Instances[info.Name()] = instance
}

func (repo *Repository) Clear() {
	repo.Instances = nil
}
