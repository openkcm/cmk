package tasks

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/repo"
)

type WorkflowRemoval interface {
	CleanupTerminalWorkflows(ctx context.Context) error
}

type WorkflowCleaner struct {
	workflowRemoval WorkflowRemoval
	repo            repo.Repo
}

func NewWorkflowCleaner(
	workflowRemoval WorkflowRemoval,
	repo repo.Repo,
	opts ...async.TaskOption,
) async.TenantTaskHandler {
	wc := &WorkflowCleaner{
		workflowRemoval: workflowRemoval,
		repo:            repo,
	}

	for _, o := range opts {
		o(wc)
	}

	return wc
}

func (wc *WorkflowCleaner) ProcessTask(ctx context.Context, task *asynq.Task) error {
	return wc.workflowRemoval.CleanupTerminalWorkflows(ctx)
}

func (wc *WorkflowCleaner) TenantQuery() *repo.Query {
	return repo.NewQuery()
}

func (wc *WorkflowCleaner) FanOutFunc() async.FanOutFunc {
	return async.TenantFanOut
}

func (wc *WorkflowCleaner) TaskType() string {
	return config.TypeWorkflowCleanup
}
