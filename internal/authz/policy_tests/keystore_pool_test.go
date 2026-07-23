package authz_policy_test

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async/tasks"
	"github.com/openkcm/cmk/internal/auditor"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// TestKeystorePool_AuthzPolicy verifies that the InternalTaskKeystorePoolRole policy
// grants exactly the repo access that ProviderConfigManager.FillKeystorePool requires,
// without the manager being mocked out.
//
// With pool size set to 0, FillKeystorePool only calls Pool.Count (Count on Keystore)
// then exits without creating any keystores — keeping the test free of keystore plugin
// dependencies while still exercising the authz layer.
func TestKeystorePool_AuthzPolicy(t *testing.T) {
	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	})
	tenant := tenants[0]
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	ctx, err := cmkcontext.InjectInternalUserData(ctx, constants.InternalTaskKeystorePoolRole)
	assert.NoError(t, err)

	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(), r, &config.Config{})
	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	ps := testutils.NewTestPlugins(
		testplugins.WithCertificateIssuer(testplugins.NewTestCertificateIssuer()),
		testplugins.WithKeystoreManagement(testplugins.Name, testplugins.NewTestKeystoreManagement()),
	)
	cfg := &config.Config{
		Database: dbCfg,
		// Size 0: FillKeystorePool calls Count then immediately returns — no Create needed.
		KeystorePool: config.KeystorePool{Size: 0},
	}

	eventFactory, err := eventprocessor.NewEventFactory(t.Context(), cfg, r)
	assert.NoError(t, err)

	cmkAuditor := auditor.New(t.Context(), cfg)
	userManager := manager.NewUserManager(authzRepo, cmkAuditor)
	certManager := manager.NewCertificateManager(t.Context(), authzRepo, ps, cfg)
	resourceLabelManager := manager.NewResourceLabelManager(authzRepo)
	tagManager := manager.NewTagManager(resourceLabelManager)
	tenantConfigManager := manager.NewTenantConfigManager(authzRepo, ps, cfg)
	keyConfigManager := manager.NewKeyConfigManager(authzRepo, certManager, userManager, tagManager, cmkAuditor, eventFactory, cfg)
	keyManager := manager.NewKeyManager(
		authzRepo,
		ps,
		tenantConfigManager,
		keyConfigManager,
		userManager,
		certManager,
		eventFactory,
		cmkAuditor,
	)

	filler := tasks.NewKeystorePoolFiller(keyManager, authzRepo, cfg.KeystorePool)
	task := asynq.NewTask(config.TypeKeystorePool, nil)

	t.Run("InternalTaskKeystorePoolRole allows Count on Keystore", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		err := filler.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.NotContains(t, strings.ToLower(buf.String()), "error",
			"unexpected error log: %s", buf.String())
	})
}
