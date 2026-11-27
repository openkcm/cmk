package async_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/zeebo/assert"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	asyncUtils "github.com/openkcm/cmk/utils/async"
)

func TestBatchProcessor(t *testing.T) {
	tenantCount := 3
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	}, testutils.WithGenerateTenants(tenantCount))
	r := sql.NewRepository(db)
	task := asynq.NewTask("test", nil)

	t.Run("Should run task on one job", func(t *testing.T) {
		bp := async.NewBatchProcessor(r)
		count := 0
		err := bp.ProcessTenantsInBatch(t.Context(), task, func(ctx context.Context, _ *asynq.Task) error {
			count++
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, tenantCount, count)
	})

	t.Run("Should fan out task to #tenant jobs", func(t *testing.T) {
		client := &async.MockClient{}
		bp := async.NewBatchProcessor(r, async.WithFanOutTenants(client))
		count := 0
		err := bp.ProcessTenantsInBatch(t.Context(), task, func(ctx context.Context, _ *asynq.Task) error {
			count++
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 0, count)
		assert.Equal(t, tenantCount, client.EnqueueCallCount)
	})

	//nolint:err113
	t.Run("Should continue executing tenants if one fails", func(t *testing.T) {
		bp := async.NewBatchProcessor(r)
		count := 0
		err := bp.ProcessTenantsInBatch(t.Context(), task,
			func(ctx context.Context, _ *asynq.Task) error {
				count++
				return errors.New("err")
			})
		assert.NoError(t, err)
		assert.Equal(t, tenantCount, count)
	})

	t.Run("Should apply on tenants in payload", func(t *testing.T) {
		bp := async.NewBatchProcessor(r)
		payload, err := json.Marshal(asyncUtils.NewTenantListPayload([]string{"tenant1"}))
		assert.NoError(t, err)
		task := asynq.NewTask("tenant1", payload)
		count := 0
		err = bp.ProcessTenantsInBatch(t.Context(), task, func(ctx context.Context, _ *asynq.Task) error {
			count++
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("Should reject unknown tenant ID in payload", func(t *testing.T) {
		bp := async.NewBatchProcessor(r)
		payload, err := json.Marshal(asyncUtils.NewTenantListPayload([]string{"tenant1", "does-not-exist"}))
		assert.NoError(t, err)
		task := asynq.NewTask("test", payload)
		count := 0
		err = bp.ProcessTenantsInBatch(t.Context(), task, func(ctx context.Context, _ *asynq.Task) error {
			count++
			return nil
		})
		assert.Error(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Should reject empty tenant ID in payload", func(t *testing.T) {
		bp := async.NewBatchProcessor(r)
		payload, err := json.Marshal(asyncUtils.NewTenantListPayload([]string{""}))
		assert.NoError(t, err)
		task := asynq.NewTask("test", payload)
		count := 0
		err = bp.ProcessTenantsInBatch(t.Context(), task, func(ctx context.Context, _ *asynq.Task) error {
			count++
			return nil
		})
		assert.Error(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Should accept duplicate valid tenant IDs in payload", func(t *testing.T) {
		bp := async.NewBatchProcessor(r)
		payload, err := json.Marshal(asyncUtils.NewTenantListPayload([]string{"tenant1", "tenant1"}))
		assert.NoError(t, err)
		task := asynq.NewTask("test", payload)
		count := 0
		err = bp.ProcessTenantsInBatch(t.Context(), task, func(ctx context.Context, _ *asynq.Task) error {
			count++
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("Should process all tenants on empty tenant list", func(t *testing.T) {
		bp := async.NewBatchProcessor(r)
		payload, err := json.Marshal(asyncUtils.NewTenantListPayload([]string{}))
		assert.NoError(t, err)
		task := asynq.NewTask("test", payload)
		count := 0
		err = bp.ProcessTenantsInBatch(t.Context(), task, func(ctx context.Context, _ *asynq.Task) error {
			count++
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, tenantCount, count)
	})
}
