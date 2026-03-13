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
		err := async.FanOutTask(client, task, asyncUtils.NewTaskPayload(t.Context(), []byte("a")))
		assert.NoError(t, err)
	}
	assert.Equal(t, count, client.EnqueueCallCount)
}

func TestIsChildTask(t *testing.T) {
	task := asynq.NewTask("test:child", []byte(""))
	assert.True(t, async.IsChildTask(task))
}

func TestProcessChild(t *testing.T) {
	t.Run("Should fail on invalid payload", func(t *testing.T) {
		err := async.ProcessChildTask(t.Context(), &asynq.Task{}, func(ctx context.Context, _ *asynq.Task) error {
			return nil
		})
		assert.Error(t, err)
	})

	t.Run("Should execute", func(t *testing.T) {
		payload := asyncUtils.NewTaskPayload(t.Context(), []byte("test-message"))
		payloadBytes, err := payload.ToBytes()
		assert.NoError(t, err)
		task := asynq.NewTask("test", payloadBytes)

		err = async.ProcessChildTask(t.Context(), task, func(ctx context.Context, _ *asynq.Task) error {
			return nil
		})
		assert.NoError(t, err)
	})
}
