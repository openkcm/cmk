package async_test

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/zeebo/assert"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	asyncUtils "github.com/openkcm/cmk/utils/async"
)

func TestFanOutCreation(t *testing.T) {
	client := &async.MockClient{}
	task := asynq.NewTask("test", []byte("test-message"))
	count := 3
	for range count {
		err := async.FanOutTask(t.Context(), client, task, asyncUtils.NewTaskPayload(t.Context(), []byte("a")))
		assert.NoError(t, err)
	}
	assert.Equal(t, count, client.EnqueueCallCount)
}

func TestTenantFanOut(t *testing.T) {
	async.SetTenantValidationRepo(nil)
	t.Cleanup(func() { async.SetTenantValidationRepo(nil) })

	t.Run("Should accept a non-empty tenant when no validation repo is set", func(t *testing.T) {
		payload := asyncUtils.TaskPayload{
			TenantID: "tenant-ctx",
		}
		bytes, err := payload.ToBytes()
		assert.NoError(t, err)

		err = async.TenantFanOut(
			t.Context(),
			asynq.NewTask("test", bytes),
			func(ctx context.Context, task *asynq.Task) error {
				return nil
			},
		)
		assert.NoError(t, err)
	})

	t.Run("Should inject payload", func(t *testing.T) {
		err := async.TenantFanOut(
			t.Context(),
			&asynq.Task{},
			func(ctx context.Context, task *asynq.Task) error {
				return nil
			},
		)
		assert.Error(t, err)
	})

	t.Run("Should reject empty tenant ID", func(t *testing.T) {
		payload := asyncUtils.TaskPayload{TenantID: ""}
		bytes, err := payload.ToBytes()
		assert.NoError(t, err)

		called := false
		err = async.TenantFanOut(
			t.Context(),
			asynq.NewTask("test", bytes),
			func(ctx context.Context, task *asynq.Task) error {
				called = true
				return nil
			},
		)
		assert.Error(t, err)
		assert.False(t, called)
	})
}

func TestTenantFanOutExistenceCheck(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	}, testutils.WithGenerateTenants(1))
	async.SetTenantValidationRepo(sql.NewRepository(db))
	t.Cleanup(func() { async.SetTenantValidationRepo(nil) })

	run := func(t *testing.T, tenantID string) (bool, error) {
		t.Helper()
		payload := asyncUtils.TaskPayload{TenantID: tenantID}
		bytes, err := payload.ToBytes()
		assert.NoError(t, err)

		called := false
		err = async.TenantFanOut(
			t.Context(),
			asynq.NewTask("test", bytes),
			func(ctx context.Context, task *asynq.Task) error {
				called = true
				return nil
			},
		)
		return called, err
	}

	t.Run("Should run for known tenant", func(t *testing.T) {
		called, err := run(t, tenants[0])
		assert.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("Should reject unknown tenant", func(t *testing.T) {
		called, err := run(t, "does-not-exist")
		assert.Error(t, err)
		assert.False(t, called)
	})
}
