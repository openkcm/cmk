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
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// TestSystemRefresh_AuthzPolicy verifies that the InternalTaskSystemRefreshRole
// policy grants exactly the repo access that SystemInformation.UpdateSystems
// requires, without the manager being mocked out.
//
// No systems are seeded, so UpdateSystems exits after the Count+List authz
// checks with an empty batch — confirming those operations are permitted.
func TestSystemRefresh_AuthzPolicy(t *testing.T) {
	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	})
	tenant := tenants[0]
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	ctx, err := cmkcontext.InjectInternalUserData(ctx, constants.InternalTaskSystemRefreshRole)
	assert.NoError(t, err)

	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(), r, &config.Config{})
	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	ps := testutils.NewTestPlugins(
		testplugins.WithCertificateIssuer(testplugins.NewTestCertificateIssuer()),
		testplugins.WithSystemInformation(testplugins.NewTestSystemInformation()),
	)
	cfg := &config.Config{
		Database: dbCfg,
	}

	siManager, err := manager.NewSystemInformationManager(authzRepo, authzRepoLoader, ps, &cfg.ContextModels.System)
	assert.NoError(t, err)

	refresher := tasks.NewSystemsRefresher(siManager, authzRepo)
	task := asynq.NewTask(config.TypeSystemsTask, nil)

	// No systems are seeded — UpdateSystems calls ProcessInBatch → Count+List on
	// System → empty batch → clean exit. This confirms the policy permits those
	// operations.
	t.Run("InternalTaskSystemRefreshRole allows Count and List on System", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		err := refresher.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.NotContains(t, strings.ToLower(buf.String()), "error",
			"unexpected error log: %s", buf.String())
	})
}
