package async

import (
	"context"
	"strings"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/log"
	asyncUtils "github.com/openkcm/cmk/utils/async"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// FanOutTask enqueues a child task for a task
// This is used to allow parallelism on tasks running in a loop manner
// Example of it's usage is for example in the ProcessTenantsInBatch
// to spawn a task for each tenant
func FanOutTask(
	asyncClient Client,
	parentTask *asynq.Task,
	payload asyncUtils.TaskPayload,
	opts ...asynq.Option,
) error {
	// Determine child task type
	childTaskType := parentTask.Type() + ":child"

	payloadBytes, err := payload.ToBytes()
	if err != nil {
		return err
	}

	childTask := asynq.NewTask(childTaskType, payloadBytes, opts...)
	_, err = asyncClient.Enqueue(childTask)
	if err != nil {
		return ErrEnqueueingTask
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

func IsChildTask(task *asynq.Task) bool {
	return strings.HasSuffix(task.Type(), ":child")
}

func ProcessChildTask(ctx context.Context, task *asynq.Task, f func(ctx context.Context) error) error {
	payload, err := asyncUtils.ParseTaskPayload(task.Payload())
	if err != nil {
		log.Error(ctx, "Failed to parse tenant from child task payload", err)
		return err
	}

	ctx = cmkcontext.New(
		ctx,
		cmkcontext.WithTenant(payload.TenantID),
		cmkcontext.InjectSystemUser,
	)

	err = f(ctx)
	if err != nil {
		log.Error(ctx, "Error processing tenant in child task", err)
		return err
	}

	return nil
}

func (c *ChildTaskWrapper) ProcessTask(ctx context.Context, task *asynq.Task) error {
	return c.parentHandler.ProcessTask(ctx, task)
}

func (c *ChildTaskWrapper) TaskType() string {
	return c.childTaskType
}

func (c *ChildTaskWrapper) SetFanOut(client Client, opts ...asynq.Option) {
}

func (c *ChildTaskWrapper) IsFanOutEnabled() bool {
	return false
}
