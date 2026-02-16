package identity_management

import (
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
)

type Repository struct {
	Instance identitymanagement.IdentityManagement
}

func (repo *Repository) IdentityManagement() (identitymanagement.IdentityManagement, bool) {
	return repo.Instance, repo.Instance != nil
}

func (repo *Repository) SetIdentityManagement(instance identitymanagement.IdentityManagement) {
	repo.Instance = instance
}

func (repo *Repository) Clear() {
	repo.Instance = nil
}
