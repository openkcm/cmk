package authz_policy_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// TestBatchProcessor_AuthzPolicy verifies that the InternalTaskProcessingRole policy
// grants the repo access that BatchProcessor.ProcessTenantsInBatch requires, without
// any managers being mocked out.
//
// ProcessTenantsInBatch injects InternalTaskProcessingRole then calls ProcessInBatch
// → Count+List on Tenant. No tenants are seeded, so it exits after those authz
// checks with an empty batch — confirming those operations are permitted.
func TestBatchProcessor_AuthzPolicy(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	})
	tenant := tenants[0]
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(), r, &config.Config{})
	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	bp := async.NewBatchProcessor(authzRepo)
	task := asynq.NewTask("test:task", nil)

	// No tenants are seeded beyond the default test tenant. ProcessTenantsInBatch
	// injects InternalTaskProcessingRole and calls ProcessInBatch → Count+List on
	// Tenant → empty batch → clean exit.
	t.Run("InternalTaskProcessingRole allows Count and List on Tenant", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		err := bp.ProcessTenantsInBatch(ctx, task, func(_ context.Context, _ *asynq.Task) error {
			return nil
		})
		assert.NoError(t, err)
		assert.NotContains(t, strings.ToLower(buf.String()), "error",
			"unexpected error log: %s", buf.String())
	})
}
