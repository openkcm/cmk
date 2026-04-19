package async_test

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/zeebo/assert"

	"github.com/openkcm/cmk/internal/async"
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
	t.Run("Should error on invalid payload", func(t *testing.T) {
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
}
