package testutils

import (
	"context"

	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/repo"
)

const (
	RepoResourceTypeTest authz.RepoResourceType = "test"
	TestAdminRole        constants.BusinessRole = "TEST_ADMIN"
	TestReadAllowedRole  constants.BusinessRole = "TEST_READ_ALLOWED"
	TestWriteAllowedRole constants.BusinessRole = "TEST_WRITE_ALLOWED"
	TestBlockedRole      constants.BusinessRole = "TEST_BLOCKED"
)

var RepoResourceTypeActions = map[authz.RepoResourceType][]authz.RepoAction{
	RepoResourceTypeTest: {
		authz.RepoActionList,
		authz.RepoActionFirst,
		authz.RepoActionCount,
		authz.RepoActionCreate,
		authz.RepoActionUpdate,
		authz.RepoActionDelete,
	},
}

var RepoActionResourceTypes map[authz.RepoAction]authz.RepoResourceType

var RepoBusinessPolicies = authz.RolePolicies[constants.BusinessRole, authz.RepoResourceType, authz.RepoAction]{
	TestAdminRole: {
		{
			ID: "ReadAdminPolicy",
			ResourceTypes: []authz.Resource[authz.RepoResourceType, authz.RepoAction]{
				{
					Type: RepoResourceTypeTest,
					Actions: []authz.RepoAction{
						authz.RepoActionList,
						authz.RepoActionFirst,
						authz.RepoActionCount,
						authz.RepoActionCreate,
						authz.RepoActionUpdate,
						authz.RepoActionDelete,
					},
				},
			},
		},
	},
	TestReadAllowedRole: {
		{
			ID: "ReadAllowedPolicy",
			ResourceTypes: []authz.Resource[authz.RepoResourceType, authz.RepoAction]{
				{
					Type: RepoResourceTypeTest,
					Actions: []authz.RepoAction{
						authz.RepoActionList,
						authz.RepoActionFirst,
						authz.RepoActionCount,
					},
				},
			},
		},
	},
	TestWriteAllowedRole: {
		{
			ID: "WriteAllowedPolicy",
			ResourceTypes: []authz.Resource[authz.RepoResourceType, authz.RepoAction]{
				{
					Type: RepoResourceTypeTest,
					Actions: []authz.RepoAction{
						authz.RepoActionCreate,
						authz.RepoActionUpdate,
						authz.RepoActionDelete,
					},
				},
			},
		},
	},
	TestBlockedRole: {
		{
			ID: "BlockedPolicy",
			ResourceTypes: []authz.Resource[authz.RepoResourceType, authz.RepoAction]{
				{
					Type:    RepoResourceTypeTest,
					Actions: []authz.RepoAction{},
				},
			},
		},
	},
}

func NewRepoAuthzLoader(
	ctx context.Context,
	r repo.Repo,
	config *config.Config,
) *authz_loader.AuthzLoader[authz.RepoResourceType, authz.RepoAction] {
	repoInternalPolicies := make(authz.RolePolicies[constants.InternalRole, authz.RepoResourceType, authz.RepoAction])
	return authz_loader.NewAuthzLoader(
		ctx,
		r,
		config,
		repoInternalPolicies,
		RepoBusinessPolicies,
		RepoResourceTypeActions,
	)
}
