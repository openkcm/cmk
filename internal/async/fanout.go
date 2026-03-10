package async

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	asyncUtils "github.com/openkcm/cmk/utils/async"
)

// NewFanOutTask registers both the parent task and its child task handler
// This is a convenience function to avoid manually registering both handlers
// Returns a slice containing both handlers ready for registration
func NewFanOutTask(handler TaskHandler) []TaskHandler {
	return []TaskHandler{
		handler,               // Parent task
		NewChildTask(handler), // Child task with ":child" suffix
	}
}

// FanOutTask enqueues a child task for a task
// This is used to allow parallelism on tasks running in a loop manner
// Example of it's usage is for example in the ProcessTenantsInBatch
// to spawn a task for each tenant
func FanOutTask(
	ctx context.Context,
	asyncClient Client,
	parentTask *asynq.Task,
	payload asyncUtils.TaskPayload,
) error {
	// Determine child task type
	childTaskType := parentTask.Type() + ":child"

	payloadBytes, err := payload.ToBytes()
	if err != nil {
		return err
	}

	childTask := asynq.NewTask(childTaskType, payloadBytes)
	_, err = asyncClient.Enqueue(childTask)
	if err != nil {
		return fmt.Errorf("failed to enqueue child task")
	}

	return nil
}

type ChildTaskWrapper struct {
	parentHandler TaskHandler
	childTaskType string
}

func NewChildTask(parentHandler TaskHandler) *ChildTaskWrapper {
	return &ChildTaskWrapper{
		parentHandler: parentHandler,
		childTaskType: parentHandler.TaskType() + ":child",
	}
}

func (c *ChildTaskWrapper) ProcessTask(ctx context.Context, task *asynq.Task) error {
	return c.parentHandler.ProcessTask(ctx, task)
}

func (c *ChildTaskWrapper) TaskType() string {
	return c.childTaskType
}
