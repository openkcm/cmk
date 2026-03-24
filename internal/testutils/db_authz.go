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
	RepoResourceTypeTest authz.RepoResourceTypeName = "test"
	TestAdminRole        constants.Role             = "TEST_ADMIN"
	TestReadAllowedRole  constants.Role             = "TEST_READ_ALLOWED"
	TestWriteAllowedRole constants.Role             = "TEST_WRITE_ALLOWED"
	TestBlockedRole      constants.Role             = "TEST_BLOCKED"
)

var RepoResourceTypeActions = map[authz.RepoResourceTypeName][]authz.RepoAction{
	RepoResourceTypeTest: {
		authz.RepoActionList,
		authz.RepoActionFirst,
		authz.RepoActionCount,
		authz.RepoActionCreate,
		authz.RepoActionUpdate,
		authz.RepoActionDelete,
	},
}

var RepoActionResourceTypes map[authz.RepoAction]authz.RepoResourceTypeName

var RepoRolePolicies = make(map[constants.Role][]authz.BasePolicy[authz.RepoResourceTypeName,
	authz.RepoAction])

type repoPolicies struct {
	Roles    []constants.Role
	Policies []authz.BasePolicy[authz.RepoResourceTypeName, authz.RepoAction]
}

var RepoPolicyData = repoPolicies{
	Roles: []constants.Role{
		constants.KeyAdminRole, constants.TenantAdminRole, constants.TenantAuditorRole,
	},
	Policies: []authz.BasePolicy[authz.RepoResourceTypeName, authz.RepoAction]{
		authz.NewPolicy(
			"ReadAdminPolicy",
			TestAdminRole,
			[]authz.BaseResourceType[authz.RepoResourceTypeName, authz.RepoAction]{
				{
					ID: RepoResourceTypeTest,
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
		),
		authz.NewPolicy(
			"ReadAllowedPolicy",
			TestReadAllowedRole,
			[]authz.BaseResourceType[authz.RepoResourceTypeName, authz.RepoAction]{
				{
					ID: RepoResourceTypeTest,
					Actions: []authz.RepoAction{
						authz.RepoActionList,
						authz.RepoActionFirst,
						authz.RepoActionCount,
					},
				},
			},
		),
		authz.NewPolicy(
			"WriteAllowedPolicy",
			TestWriteAllowedRole,
			[]authz.BaseResourceType[authz.RepoResourceTypeName, authz.RepoAction]{
				{
					ID: RepoResourceTypeTest,
					Actions: []authz.RepoAction{
						authz.RepoActionCreate,
						authz.RepoActionUpdate,
						authz.RepoActionDelete,
					},
				},
			},
		),
		authz.NewPolicy(
			"BlockedPolicy",
			TestBlockedRole,
			[]authz.BaseResourceType[authz.RepoResourceTypeName, authz.RepoAction]{
				{
					ID:      RepoResourceTypeTest,
					Actions: []authz.RepoAction{},
				},
			},
		),
	},
}

func NewRepoAuthzLoader(
	ctx context.Context,
	r repo.Repo,
	config *config.Config,
) *authz_loader.AuthzLoader[authz.RepoResourceTypeName, authz.RepoAction] {
	return authz_loader.NewAuthzLoader(ctx, r, config,
		RepoRolePolicies, RepoResourceTypeActions)
}

func init() {
	// Index policies by role for fast lookup
	RepoRolePolicies = make(map[constants.Role][]authz.BasePolicy[authz.RepoResourceTypeName,
		authz.RepoAction])
	for _, policy := range RepoPolicyData.Policies {
		RepoRolePolicies[policy.Role] = append(RepoRolePolicies[policy.Role], policy)
	}
}
