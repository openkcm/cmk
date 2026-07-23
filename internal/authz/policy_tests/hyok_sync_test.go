package authz_policy_test

import (
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	tasks "github.com/openkcm/cmk/internal/async/tasks/tenant"
	"github.com/openkcm/cmk/internal/auditor"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

// TestHYOKSync_AuthzPolicy verifies that the InternalTaskHYOKSyncRole policy
// grants exactly the repo access that KeyManager.SyncHYOKKeys requires, without
// the manager being mocked out.
//
// A HYOK key and a tenant-default certificate are seeded so that SyncHYOKKeys
// exercises the full path through GetOrInitProvider → getDefaultHYOKClientCert,
// which requires Certificate permissions beyond just Key Count+List.
func TestHYOKSync_AuthzPolicy(t *testing.T) {
	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	})
	tenant := tenants[0]
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	ctx, err := cmkcontext.InjectInternalUserData(ctx, constants.InternalTaskHYOKSyncRole)
	assert.NoError(t, err)

	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(), r, &config.Config{})
	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	ps := testutils.NewTestPlugins(testplugins.WithCertificateIssuer(testplugins.NewTestCertificateIssuer()))
	cfg := &config.Config{
		Database: dbCfg,
	}

	eventFactory, err := eventprocessor.NewEventFactory(t.Context(), cfg, r)
	assert.NoError(t, err)

	cmkAuditor := auditor.New(t.Context(), cfg)
	certManager := manager.NewCertificateManager(t.Context(), authzRepo, ps, cfg)
	tenantConfigManager := manager.NewTenantConfigManager(authzRepo, ps, cfg)
	tagManager := manager.NewTagManager(authzRepo)
	userManager := manager.NewUserManager(authzRepo, cmkAuditor)
	keyConfigManager := manager.NewKeyConfigManager(authzRepo, certManager, userManager, tagManager, cmkAuditor, eventFactory, cfg)

	keyManager := manager.NewKeyManager(
		authzRepo,
		ps,
		tenantConfigManager,
		keyConfigManager,
		userManager,
		certManager,
		nil, // eventFactory
		cmkAuditor,
	)

	// Create a tenant-default certificate and a HYOK key
	hyokInfo, err := json.Marshal(testutils.ValidKeystoreAccountInfo)
	assert.NoError(t, err)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	cert := testutils.NewCertificate(func(_ *model.Certificate) {})
	hyokKey := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfig.ID
		k.KeyType = cmkapi.KeyTypeHYOK
		k.NativeID = ptr.PointTo("mock-key/11111111")
		k.ManagementAccessData = hyokInfo
		k.Provider = testplugins.Name
	})
	testutils.CreateTestEntities(ctx, t, r, keyConfig, cert, hyokKey)

	hyokSync := tasks.NewHYOKSync(keyManager, authzRepo)
	task := asynq.NewTask(config.TypeHYOKSync, nil)

	t.Run("InternalTaskHYOKSyncRole allows full HYOK sync path including Certificate access", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		err := hyokSync.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.NotContains(t, strings.ToLower(buf.String()), "error",
			"unexpected error log: %s", buf.String())
	})
}
