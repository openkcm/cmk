package authz_policy_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/openkcm/plugin-sdk/api"
	"github.com/stretchr/testify/assert"

	tasks "github.com/openkcm/cmk/internal/async/tasks/tenant"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/systeminformation"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

const systemRoleNameKey = "roleName"

// fakeSystemInformation returns fixed metadata for every system so that
// UpdateSystems reaches the read-and-persist path instead of exiting early on
// nil metadata.
type fakeSystemInformation struct {
	metadata map[string]string
}

var _ systeminformation.SystemInformation = (*fakeSystemInformation)(nil)

func (s *fakeSystemInformation) ServiceInfo() api.Info {
	return testplugins.NewTestSystemInformation().ServiceInfo()
}

func (s *fakeSystemInformation) GetSystemInfo(
	_ context.Context,
	_ *systeminformation.GetSystemInfoRequest,
) (*systeminformation.GetSystemInfoResponse, error) {
	return &systeminformation.GetSystemInfoResponse{Metadata: s.metadata}, nil
}

// TestSystemRefresh_AuthzPolicy verifies that InternalTaskSystemRefreshRole
// grants exactly the repo access SystemInformation.UpdateSystems requires,
// without mocking the manager.
func TestSystemRefresh_AuthzPolicy(t *testing.T) {
	t.Run("allows Count and List on System", func(t *testing.T) {
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
		cfg := &config.Config{Database: dbCfg}

		siManager, err := manager.NewSystemInformationManager(authzRepo, authzRepoLoader, ps, &cfg.ContextModels.System)
		assert.NoError(t, err)

		refresher := tasks.NewSystemsRefresher(siManager, authzRepo)
		task := asynq.NewTask(config.TypeSystemsTask, nil)

		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		// No systems seeded: Count+List on System returns an empty batch and exits.
		err = refresher.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.NotContains(t, strings.ToLower(buf.String()), "error",
			"unexpected error log: %s", buf.String())
	})

	t.Run("allows First and Update on System", func(t *testing.T) {
		db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})
		tenant := tenants[0]
		ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
		internalCtx, err := cmkcontext.InjectInternalUserData(ctx, constants.InternalTaskSystemRefreshRole)
		assert.NoError(t, err)

		r := sql.NewRepository(db)

		system := testutils.NewSystem(func(s *model.System) {
			s.Status = "DISCONNECTED"
		})
		testutils.CreateTestEntities(ctx, t, r, system)

		authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(), r, &config.Config{})
		authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

		// The property key must be configured as optional, otherwise the manager
		// ignores it and never persists an update.
		plugin := &fakeSystemInformation{metadata: map[string]string{systemRoleNameKey: "SomeRole"}}
		ps := testutils.NewTestPlugins(
			testplugins.WithCertificateIssuer(testplugins.NewTestCertificateIssuer()),
			testplugins.WithSystemInformation(plugin),
		)
		cfg := &config.Config{Database: dbCfg}
		cfg.ContextModels.System.OptionalProperties = map[string]config.SystemProperty{
			systemRoleNameKey: {},
		}

		siManager, err := manager.NewSystemInformationManager(authzRepo, authzRepoLoader, ps, &cfg.ContextModels.System)
		assert.NoError(t, err)

		refresher := tasks.NewSystemsRefresher(siManager, authzRepo)
		task := asynq.NewTask(config.TypeSystemsTask, nil)

		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		// A seeded system drives First (read with properties) and Update (patch).
		err = refresher.ProcessTask(internalCtx, task)
		assert.NoError(t, err)
		assert.NotContains(t, strings.ToLower(buf.String()), "error",
			"unexpected error log: %s", buf.String())

		// The persisted property proves the authz-guarded patch succeeded.
		updated, err := repo.GetSystemByIDWithProperties(ctx, r, system.ID, repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, "SomeRole", updated.Properties[systemRoleNameKey])
	})
}
