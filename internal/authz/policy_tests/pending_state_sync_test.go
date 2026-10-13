package authz_policy_test

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/async/tasks"
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
	asyncUtils "github.com/openkcm/cmk/utils/async"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// TestPendingStateSync_AuthzPolicy verifies that InternalTaskPendingStateSyncRole
// grants sufficient repo access for KeyManager.SyncPendingCreationKey to reach the
// keystore layer without hitting an authorization denial.
//
// A BYOK key in PENDING_CREATION state is seeded (no stored keystore config). The
// sync call traverses GetOrInitProvider → GetDefaultKeystoreConfig →
// GetStoredDefaultKeystoreConfig → listConfigsByType (TenantConfig:List), then falls
// back to the keystore pool. With an empty pool the key transitions to ERROR via
// ErrPoolIsDrained — confirming authz passed through the entire path.
func TestPendingStateSync_AuthzPolicy(t *testing.T) {
	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	})
	tenant := tenants[0]
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	ctx, err := cmkcontext.InjectInternalUserData(ctx, constants.InternalTaskPendingStateSyncRole)
	require.NoError(t, err)

	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(), r, &config.Config{})
	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	ps := testutils.NewTestPlugins(
		testplugins.WithCertificateIssuer(testplugins.NewTestCertificateIssuer()),
		testplugins.WithKeystoreManagement(testplugins.Name, testplugins.NewTestKeystoreManagement()),
	)
	cfg := &config.Config{
		Database: dbCfg,
		// Empty pool: GetDefaultKeystoreConfig falls through to pool, gets ErrPoolIsDrained.
		KeystorePool: config.KeystorePool{Size: 0},
	}

	eventFactory, err := eventprocessor.NewEventFactory(t.Context(), cfg, r)
	require.NoError(t, err)

	cmkAuditor := auditor.New(t.Context(), cfg)
	certManager := manager.NewCertificateManager(t.Context(), authzRepo, ps, cfg)
	tenantConfigManager := manager.NewTenantConfigManager(authzRepo, ps, cfg, certManager, nil)
	tagManager := manager.NewTagManager(authzRepo)
	userManager := manager.NewUserManager(authzRepo, cmkAuditor)
	keyConfigManager := manager.NewKeyConfigManager(authzRepo, certManager, userManager, tagManager, cmkAuditor, eventFactory, cfg, nil)
	keyManager := manager.NewKeyManager(
		authzRepo,
		ps,
		tenantConfigManager,
		keyConfigManager,
		userManager,
		certManager,
		eventFactory,
		cmkAuditor,
		nil,
		nil,
	)

	handler := tasks.NewPendingStateSync(keyManager, authzRepo)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfig.ID
		k.KeyType = cmkapi.KeyTypeBYOK
		k.State = cmkapi.KeyStatePENDINGCREATION
	})
	testutils.CreateTestEntities(ctx, t, r, keyConfig, key)

	payload := asyncUtils.NewTaskPayload(ctx, []byte(key.ID.String()))
	payloadBytes, err := payload.ToBytes()
	require.NoError(t, err)
	task := asynq.NewTask(config.TypePendingStateSync, payloadBytes)

	t.Run("InternalTaskPendingStateSyncRole allows TenantConfig:List in BYOK provisioning path", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		// SyncPendingCreationKey gets past the TenantConfig:List authz check and
		// terminates at ErrPoolIsDrained (empty pool), not at authorization denied.
		_ = handler.ProcessTask(ctx, task)

		assert.NotContains(t, strings.ToLower(buf.String()), `"allowed":false`,
			"unexpected authorization denial in logs: %s", buf.String())
		assert.NotContains(t, strings.ToLower(buf.String()), "authorization denied",
			"unexpected authorization denial in logs: %s", buf.String())
	})
}
