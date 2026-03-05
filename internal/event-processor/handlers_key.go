package eventprocessor

import (
	"context"

	"github.com/openkcm/orbital"
)

type KeyJobHandler struct {
	taskResolver *KeyTaskInfoResolver
}

func NewKeyJobHandler(taskResolver *KeyTaskInfoResolver) *KeyJobHandler {
	return &KeyJobHandler{
		taskResolver: taskResolver,
	}
}

func (h *KeyJobHandler) ResolveTasks(
	ctx context.Context,
	job orbital.Job,
) ([]orbital.TaskInfo, error) {
	return h.taskResolver.Resolve(ctx, job)
}

func (h *KeyJobHandler) HandleJobConfirm(
	_ context.Context,
	_ orbital.Job,
) (orbital.JobConfirmerResult, error) {
	return orbital.CompleteJobConfirmer(), nil
}

func (h *KeyJobHandler) HandleJobDoneEvent(
	_ context.Context,
	_ orbital.Job,
) error {
	return nil
}

func (h *KeyJobHandler) HandleJobFailedEvent(
	_ context.Context,
	_ orbital.Job,
) error {
	return nil
}

func (h *KeyJobHandler) HandleJobCanceledEvent(
	_ context.Context,
	_ orbital.Job,
) error {
	return nil
}
