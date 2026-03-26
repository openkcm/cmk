package async_test

import (
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
