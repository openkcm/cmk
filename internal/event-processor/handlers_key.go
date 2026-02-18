package eventprocessor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openkcm/orbital"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/event-processor/proto"
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
	data, err := h.UnmarshalJobData(orbital.Job{Data: job.Data})
	if err != nil {
		return nil, err
	}

	var taskType proto.TaskType
	switch JobType(job.Type) {
	case JobTypeKeyEnable:
		taskType = proto.TaskType_KEY_ENABLE
	case JobTypeKeyDisable:
		taskType = proto.TaskType_KEY_DISABLE
	case JobTypeKeyDetach:
		taskType = proto.TaskType_KEY_DETACH
	case JobTypeKeyDelete:
		taskType = proto.TaskType_KEY_DELETE
	case JobTypeKeyRotate:
		taskType = proto.TaskType_KEY_ROTATE
	default:
		return nil, errs.Wrapf(ErrInvalidJobType, job.Type)
	}

	taskInfos, err := h.taskResolver.GetTaskInfo(ctx, taskType, data)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve task info: %w", err)
	}

	return taskInfos, nil
}

func (h *KeyJobHandler) HandleJobConfirm(
	_ context.Context,
	_ orbital.Job,
) (orbital.JobConfirmerResult, error) {
	return orbital.CompleteJobConfirmer(), nil
}

func (h *KeyJobHandler) UnmarshalJobData(job orbital.Job) (KeyActionJobData, error) {
	var data KeyActionJobData

	err := json.Unmarshal(job.Data, &data)
	if err != nil {
		return KeyActionJobData{}, fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	return data, nil
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
