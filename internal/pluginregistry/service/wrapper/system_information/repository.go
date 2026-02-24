package system_information

import (
	"errors"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/systeminformation"
)

var ErrNotConfigured = errors.New("system information plugin not configured")

type Repository struct {
	Instance systeminformation.SystemInformation
}

func (repo *Repository) SystemInformation() (systeminformation.SystemInformation, error) {
	if repo.Instance == nil {
		return nil, ErrNotConfigured
	}
	return repo.Instance, nil
}

func (repo *Repository) SetSystemInformation(instance systeminformation.SystemInformation) {
	repo.Instance = instance
}

func (repo *Repository) Clear() {
	repo.Instance = nil
}
