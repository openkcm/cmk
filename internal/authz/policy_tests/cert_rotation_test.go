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

// TestCertRotation_AuthzPolicy verifies that the InternalTaskCertRotationRole policy
// grants exactly the repo access that CertificateManager.RotateExpiredCertificates
// requires, without the manager being mocked out.
//
// The test wires the real CertificateManager through a real AuthzRepo so that
// every repo call (Count, List on Certificate) goes through the authz layer.
// No certificates matching the rotation predicate are seeded, so the manager
// exits cleanly after the initial Count+List without needing to issue a real
// certificate — keeping the test free of cert-issuer dependencies.
func TestCertRotation_AuthzPolicy(t *testing.T) {
	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	})
	tenant := tenants[0]
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	ctx, err := cmkcontext.InjectInternalUserData(ctx, constants.InternalTaskCertRotationRole)
	assert.NoError(t, err)

	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(), r, &config.Config{})
	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	ps := testutils.NewTestPlugins(testplugins.WithCertificateIssuer(testplugins.NewTestCertificateIssuer()))
	cfg := &config.Config{
		Database: dbCfg,
	}

	certManager := manager.NewCertificateManager(t.Context(), authzRepo, ps, cfg)
	rotator := tasks.NewCertRotator(certManager, authzRepo)
	task := asynq.NewTask(config.TypeCertificateTask, nil)

	// No certs are seeded that match the rotation predicate (AutoRotate=true AND
	// ExpirationDate < threshold), so RotateExpiredCertificates exits after the
	// Count+List authz checks without attempting to issue any certificate.
	// This is sufficient to prove the policy permits those operations.
	t.Run("InternalTaskCertRotationRole allows Count and List on Certificate", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		err := rotator.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.NotContains(t, strings.ToLower(buf.String()), "error",
			"unexpected error log: %s", buf.String())
	})
}
