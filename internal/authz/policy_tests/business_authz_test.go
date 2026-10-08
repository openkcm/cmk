package authz_policy_test

import (
	"log/slog"
	"testing"

	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// TestBusinessAuthz_AuthzPolicy verifies that the InternalBusinessAuthzRole policy
// grants the repo access that UserManager.NeedsGroupFiltering requires, without
// the manager being mocked out.
//
// NeedsGroupFiltering calls BusinessToInternalContext to switch to
// InternalBusinessAuthzRole, then calls repo.Count on Group (and optionally
// repo.List on Group via GetRoleFromIAM). No groups matching the IAM identifier
// are seeded, so both calls return cleanly with zero results — confirming Count
// and List on Group are permitted by the policy.
func TestBusinessAuthz_AuthzPolicy(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	})
	tenant := tenants[0]

	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(), r, &config.Config{})
	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	userManager := manager.NewUserManager(authzRepo, nil)

	// Build a business user context carrying an IAM group identifier. The identifier
	// does not match any seeded group, so NeedsGroupFiltering reaches the Count on
	// Group (via BusinessToInternalContext → InternalBusinessAuthzRole) and returns
	// false, nil cleanly.
	businessCtx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	businessCtx = cmkcontext.InjectBusinessUserData(businessCtx, &auth.ClientData{
		Groups: []string{"iam-group-id"},
	}, nil)

	t.Run("InternalBusinessAuthzRole allows Count and List on Group", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		needs, err := userManager.NeedsGroupFiltering(businessCtx, authz.APIActionRead, authz.APIResourceTypeKey)
		assert.NoError(t, err)
		// No groups match the IAM identifier — group filtering is needed (true) but
		// the call itself must not produce an authz error. The important thing here is
		// that the repo Count on Group was reached and permitted.
		assert.True(t, needs)
		assert.NotContains(t, buf.String(), `"allowed":false`,
			"authz denial in log — policy is missing a required permission: %s", buf.String())
	})
}
