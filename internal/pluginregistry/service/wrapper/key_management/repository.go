package key_management

import (
	"errors"
	"log/slog"

	keymanagementapi "github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
)

var ErrNotConfigured = errors.New("key management plugin not configured")

type Repository struct {
	Instances map[string]keymanagementapi.KeyManagement
}

func (repo *Repository) KeyManagements() (map[string]keymanagementapi.KeyManagement, error) {
	if len(repo.Instances) == 0 {
		return nil, ErrNotConfigured
	}
	return repo.Instances, nil
}

func (repo *Repository) KeyManagementList() ([]keymanagementapi.KeyManagement, error) {
	if len(repo.Instances) == 0 {
		return nil, ErrNotConfigured
	}

	list := make([]keymanagementapi.KeyManagement, 0, len(repo.Instances))
	for _, manager := range repo.Instances {
		list = append(list, manager)
	}
	return list, nil
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
