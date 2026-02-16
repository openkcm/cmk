package system_information

import (
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/systeminformation"
)

type Repository struct {
	Instance systeminformation.SystemInformation
}

func (repo *Repository) SystemInformation() (systeminformation.SystemInformation, bool) {
	return repo.Instance, repo.Instance != nil
}

func (repo *Repository) SetSystemInformation(instance systeminformation.SystemInformation) {
	repo.Instance = instance
}

func (repo *Repository) Clear() {
	repo.Instance = nil
}
