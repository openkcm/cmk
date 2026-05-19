package authz_policy_test

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	tasks "github.com/openkcm/cmk/internal/async/tasks/tenant"
	"github.com/openkcm/cmk/internal/auditor"
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

// TestWorkflowExpiry_AuthzPolicy verifies that the InternalTaskWorkflowExpirationRole
// policy grants the repo access that WorkflowManager.GetWorkflows requires (Count
// and List on Workflow), without the manager being mocked out.
//
// No workflows are seeded, so GetWorkflows exits after the Count+List authz
// checks with an empty result — confirming those operations are permitted without
// needing to enter the per-workflow expiry block.
func TestWorkflowExpiry_AuthzPolicy(t *testing.T) {
	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	})
	tenant := tenants[0]
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	ctx, err := cmkcontext.InjectInternalUserData(ctx, constants.InternalTaskWorkflowExpirationRole)
	assert.NoError(t, err)

	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(), r, &config.Config{})
	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	ps := testutils.NewTestPlugins(testplugins.WithCertificateIssuer(testplugins.NewTestCertificateIssuer()))
	cfg := &config.Config{
		Database: dbCfg,
	}

	cmkAuditor := auditor.New(t.Context(), cfg)
	tenantConfigManager := manager.NewTenantConfigManager(authzRepo, ps, cfg)
	userManager := manager.NewUserManager(authzRepo, cmkAuditor)

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

	processor := tasks.NewWorkflowExpiryProcessor(wfManager, authzRepo)
	task := asynq.NewTask(config.TypeWorkflowExpire, nil)

	// No workflows are seeded — GetWorkflows calls repo.List + repo.Count on
	// Workflow → empty result → clean exit without entering the expiry loop.
	// This is sufficient to prove the policy permits Count and List on Workflow.
	t.Run("InternalTaskWorkflowExpirationRole allows Count and List on Workflow", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		err := processor.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.NotContains(t, strings.ToLower(buf.String()), "error",
			"unexpected error log: %s", buf.String())
	})
}
