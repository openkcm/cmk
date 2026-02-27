package identity_management

import (
	"errors"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
)

var ErrNotConfigured = errors.New("identity management plugin not configured")

type Repository struct {
	Instance identitymanagement.IdentityManagement
}

func (repo *Repository) IdentityManagement() (identitymanagement.IdentityManagement, error) {
	if repo.Instance == nil {
		return nil, ErrNotConfigured
	}
	return repo.Instance, nil
}

func (repo *Repository) SetIdentityManagement(instance identitymanagement.IdentityManagement) {
	repo.Instance = instance
}

func (repo *Repository) Clear() {
	repo.Instance = nil
}
