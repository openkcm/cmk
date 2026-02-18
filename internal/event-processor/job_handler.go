package eventprocessor

import (
	"context"
	"github.com/openkcm/orbital"
)

type JobHandler interface {
	JobType() JobType
	ResolveTasks(ctx context.Context, taskType string, jobData []byte) ([]orbital.TaskInfo, error)
	Confirm(ctx context.Context, job orbital.Job) (orbital.JobConfirmResult, error)
	Terminate(ctx context.Context, job orbital.Job) error
}
