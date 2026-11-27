package authz_policy_test

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tenantTask "github.com/openkcm/cmk/internal/async/tasks/tenant"
	"github.com/openkcm/cmk/internal/auditor"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// TestHyperscalerRenew_AuthzPolicy verifies InternalTaskHyperscalerRenewRole grants the
// TenantConfig reads RenewHyperscalerLease needs, across its branches (one tenant each).
// The task swallows errors (log-and-return-nil), so error branches are asserted via the log buffer.
func TestHyperscalerRenew_AuthzPolicy(t *testing.T) {
	configured := *testutils.NewTenant(func(*model.Tenant) {})
	noHyperscaler := *testutils.NewTenant(func(*model.Tenant) {})
	noKeystore := *testutils.NewTenant(func(*model.Tenant) {})
	badCloudID := *testutils.NewTenant(func(*model.Tenant) {})

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{},
		testutils.WithInitTenants(configured, noHyperscaler, noKeystore, badCloudID))
	configuredT, noHyperscalerT, noKeystoreT, badCloudIDT := tenants[0], tenants[1], tenants[2], tenants[3]

	r := sql.NewRepository(db)
	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(), r, &config.Config{})
	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	ps := testutils.NewTestPlugins(
		testplugins.WithCertificateIssuer(testplugins.NewTestCertificateIssuer()),
		testplugins.WithKeystoreManagement(testplugins.Name, testplugins.NewTestKeystoreManagement()),
	)
	cfg := &config.Config{Database: dbCfg}

	eventFactory, err := eventprocessor.NewEventFactory(t.Context(), cfg, r)
	require.NoError(t, err)

	cmkAuditor := auditor.New(t.Context(), cfg)
	userManager := manager.NewUserManager(authzRepo, cmkAuditor)
	certManager := manager.NewCertificateManager(t.Context(), authzRepo, ps, cfg)
	tagManager := manager.NewTagManager(authzRepo)
	tenantConfigManager := manager.NewTenantConfigManager(authzRepo, ps, cfg, certManager, nil, nil)
	keyConfigManager := manager.NewKeyConfigManager(authzRepo, certManager, userManager, tagManager, cmkAuditor, eventFactory, cfg, nil)
	keyManager := manager.NewKeyManager(
		authzRepo, ps, tenantConfigManager, keyConfigManager,
		userManager, certManager, eventFactory, cmkAuditor, nil, nil, nil,
	)

	renew := tenantTask.NewHyperscalerRenew(keyManager, authzRepo)
	task := asynq.NewTask(config.TypeHyperscalerRenew, nil)

	// renewCtx returns a renewal-role context scoped to the given tenant.
	renewCtx := func(tenantSchema string) context.Context {
		c := cmkcontext.CreateTenantContext(t.Context(), tenantSchema)
		c, err := cmkcontext.InjectInternalUserData(c, constants.InternalTaskHyperscalerRenewRole)
		require.NoError(t, err)
		return c
	}

	hyperscalerCfg := model.HyperscalerConfig{"security_officer": "officer@example.com", "environment": "PROD"}
	keystoreCfg := func(roleArn string) model.KeystoreConfig {
		return model.KeystoreConfig{RoleManagementConfig: model.ManagementConfig{
			AccessData: model.KeystoreAccessData{"roleArn": roleArn},
		}}
	}

	t.Run("renews when hyperscaler and keystore configs are present", func(t *testing.T) {
		ctx := renewCtx(configuredT)
		seedTenantConfig(ctx, t, r, hyperscalerCfg)
		seedTenantConfig(ctx, t, r, keystoreCfg("arn:aws:iam::123456789012:role/kms"))

		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		require.NoError(t, renew.ProcessTask(ctx, task))
		assert.NotContains(t, strings.ToLower(buf.String()), "error",
			"unexpected error log: %s", buf.String())
	})

	t.Run("no-op when tenant has no hyperscaler config", func(t *testing.T) {
		ctx := renewCtx(noHyperscalerT)

		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		require.NoError(t, renew.ProcessTask(ctx, task))
		assert.NotContains(t, strings.ToLower(buf.String()), "error",
			"unexpected error log: %s", buf.String())
	})

	t.Run("logs error when hyperscaler config present but keystore config missing", func(t *testing.T) {
		ctx := renewCtx(noKeystoreT)
		seedTenantConfig(ctx, t, r, hyperscalerCfg)

		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		require.NoError(t, renew.ProcessTask(ctx, task))
		assert.Contains(t, strings.ToLower(buf.String()), "error",
			"expected an error log for missing keystore config")
	})

	t.Run("logs error when cloud identifier is unresolvable", func(t *testing.T) {
		ctx := renewCtx(badCloudIDT)
		seedTenantConfig(ctx, t, r, hyperscalerCfg)
		seedTenantConfig(ctx, t, r, keystoreCfg("not-an-arn"))

		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		require.NoError(t, renew.ProcessTask(ctx, task))
		assert.Contains(t, strings.ToLower(buf.String()), "error",
			"expected an error log for unresolvable cloud identifier")
	})
}

func seedTenantConfig(ctx context.Context, t *testing.T, r repo.Repo, cfg model.TenantConfiguration) {
	t.Helper()
	flattened, err := model.Flatten(cfg)
	require.NoError(t, err)
	rows := make([]repo.Resource, 0, len(flattened))
	for k, v := range flattened {
		rows = append(rows, &model.TenantConfig{Key: k, Value: fmt.Sprint(v), Type: cfg.Type()})
	}
	testutils.CreateTestEntities(ctx, t, r, rows...)
}
