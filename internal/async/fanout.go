package async

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/errs"
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
	payloadBytes, err := payload.ToBytes()
	if err != nil {
		return err
	}

	childTask := asynq.NewTask(parentTask.Type()+":child", payloadBytes, opts...)
	_, err = asyncClient.Enqueue(childTask)
	if err != nil {
		return errs.Wrap(ErrEnqueueingTask, err)
	}

	return nil
}

type (
	ProcessFunc func(ctx context.Context, task *asynq.Task) error
	FunOutFunc  func(ctx context.Context, task *asynq.Task, f ProcessFunc) error
)

// TenantFanOut extracts tenant from payload and injects it into context before executing
func TenantFanOut(ctx context.Context, task *asynq.Task, f ProcessFunc) error {
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

	return f(ctx, task)
}

// ChildTaskHandler wraps a parent handler and executes its ProcessTask with custom fanout logic
type ChildTaskHandler struct {
	parent TaskHandler
	fanOut func(ctx context.Context, task *asynq.Task, f ProcessFunc) error
}

func NewFanOutHandler(
	parent TaskHandler,
	fanOutFunc func(ctx context.Context, task *asynq.Task, f ProcessFunc) error,
) *ChildTaskHandler {
	return &ChildTaskHandler{
		parent: parent,
		fanOut: fanOutFunc,
	}
}

func (c *ChildTaskHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	return c.fanOut(ctx, task, c.parent.ProcessTask)
}

func (c *ChildTaskHandler) TaskType() string {
	return c.parent.TaskType() + ":child"
}

// IsFanOutEnabled returns false for child tasks
func (c *ChildTaskHandler) IsFanOutEnabled() bool {
	return false
}
