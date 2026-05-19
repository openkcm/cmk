package authz_policy_test

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	tasks "github.com/openkcm/cmk/internal/async/tasks/tenant"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// TestWorkflowCleanup_AuthzPolicy verifies that the InternalTaskWorkflowCleanupRole
// policy grants exactly the repo access that WorkflowManager.CleanupTerminalWorkflows
// requires, without the manager being mocked out.
//
// CleanupTerminalWorkflows first calls WorkflowConfig (First on TenantConfig), which
// is not in the cleanup role policy. If TenantConfig does not exist, GetWorkflowConfig
// falls back to SetWorkflowConfig (Create on TenantConfig) — also not in the policy.
// This test surfaces that gap: both paths require TenantConfig access that the cleanup
// role does not grant.
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

	tenantConfigManager := manager.NewTenantConfigManager(authzRepo, ps, cfg)
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

	// CleanupTerminalWorkflows calls WorkflowConfig → First on TenantConfig.
	// InternalTaskWorkflowCleanupRole does not grant access to TenantConfig, so
	// if TenantConfig is absent it will attempt a Create (also not in the policy).
	// Seed a TenantConfig with WorkflowConfigKey directly via the plain repo (bypassing
	// authz) so that the First succeeds and execution reaches the Workflow Count+List.
	workflowConfig := testutils.NewWorkflowConfig(func(_ *model.TenantConfig) {})
	testutils.CreateTestEntities(ctx, t, r, workflowConfig)

	t.Run("InternalTaskWorkflowCleanupRole allows Count, List, Delete on Workflow", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		err := cleaner.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.NotContains(t, strings.ToLower(buf.String()), "error",
			"unexpected error log: %s", buf.String())
	})
}
