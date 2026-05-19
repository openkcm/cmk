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
	TestAdminRole        constants.BusinessRole     = "TEST_ADMIN"
	TestReadAllowedRole  constants.BusinessRole     = "TEST_READ_ALLOWED"
	TestWriteAllowedRole constants.BusinessRole     = "TEST_WRITE_ALLOWED"
	TestBlockedRole      constants.BusinessRole     = "TEST_BLOCKED"
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

var RepoBusinessPolicies = make(map[constants.BusinessRole][]authz.BasePolicy[constants.BusinessRole,
	authz.RepoResourceTypeName, authz.RepoAction])

type repoPolicies struct {
	Roles    []constants.BusinessRole
	Policies []authz.BasePolicy[constants.BusinessRole, authz.RepoResourceTypeName, authz.RepoAction]
}

var RepoPolicyData = repoPolicies{
	Roles: []constants.BusinessRole{
		constants.KeyAdminRole, constants.TenantAdminRole, constants.TenantAuditorRole,
	},
	Policies: []authz.BasePolicy[constants.BusinessRole, authz.RepoResourceTypeName, authz.RepoAction]{
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
	repoInternalPolicies := make(map[constants.InternalRole][]authz.BasePolicy[constants.InternalRole,
		authz.RepoResourceTypeName, authz.RepoAction])
	return authz_loader.NewAuthzLoader(ctx, r, config,
		repoInternalPolicies, RepoBusinessPolicies, RepoResourceTypeActions)
}

func init() {
	// Index policies by role for fast lookup
	RepoBusinessPolicies = make(map[constants.BusinessRole][]authz.BasePolicy[constants.BusinessRole,
		authz.RepoResourceTypeName, authz.RepoAction])
	for _, policy := range RepoPolicyData.Policies {
		RepoBusinessPolicies[policy.Role] = append(RepoBusinessPolicies[policy.Role], policy)
	}
}
