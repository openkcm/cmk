package authz_policy_test

import (
	"log/slog"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	tasks "github.com/openkcm/cmk/internal/async/tasks/tenant"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// TestWorkflowCleanup_AuthzPolicy verifies that the InternalTaskWorkflowCleanupRole
// policy grants exactly the repo access that WorkflowManager.CleanupTerminalWorkflows
// requires, without the manager being mocked out.
//
// CleanupTerminalWorkflows calls GetWorkflowConfig (First on TenantConfig). When no
// config exists, GetWorkflowConfig falls back to SetWorkflowConfig which reads the
// tenant (First on Tenant) and then upserts the default config (Set = Delete+Create
// on TenantConfig). No TenantConfig is seeded so the full fallback path is exercised
// under the authz repo, confirming all three TenantConfig permissions are granted.
func TestWorkflowCleanup_AuthzPolicy(t *testing.T) {
	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	})
	tenant := tenants[0]
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	ctx, err := cmkcontext.InjectInternalUserData(ctx, constants.InternalTaskWorkflowCleanupRole)
	assert.NoError(t, err)

	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(), r, &config.Config{})
	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	ps := testutils.NewTestPlugins(testplugins.WithCertificateIssuer(testplugins.NewTestCertificateIssuer()))
	cfg := &config.Config{
		Database: dbCfg,
	}

	tenantConfigManager := manager.NewTenantConfigManager(authzRepo, ps, cfg, nil)
	userManager := manager.NewUserManager(authzRepo, nil)
	wfManager := manager.NewWorkflowManager(
		authzRepo,
		ps,
		nil, // keyManager
		nil, // keyConfigurationManager
		nil, // systemManager
		nil, // groupManager
		userManager,
		nil, // asyncClient
		tenantConfigManager,
		cfg,
	)

	cleaner := tasks.NewWorkflowCleaner(wfManager, authzRepo)
	task := asynq.NewTask(config.TypeWorkflowCleanup, nil)

	// No TenantConfig is seeded. GetWorkflowConfig will call repo.First (not found),
	// then fall through to SetWorkflowConfig → repo.GetTenant (Tenant:First) → repo.Set
	// (TenantConfig:Delete+Create). All three operations must be permitted by the policy.

	t.Run("InternalTaskWorkflowCleanupRole allows Count, List, Delete on Workflow", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		err := cleaner.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.NotContains(t, buf.String(), `"allowed":false`,
			"unexpected authz denial: %s", buf.String())
	})
}
