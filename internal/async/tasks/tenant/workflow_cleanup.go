package tasks

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
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
	ctx, err := cmkcontext.InjectInternalClientData(ctx,
		constants.InternalTaskWorkflowCleanupRole)
	if err != nil {
		wc.logError(ctx, err)
		return nil
	}

	err = wc.workflowRemoval.CleanupTerminalWorkflows(ctx)
	if err != nil {
		wc.logError(ctx, err)
	}
	return nil
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

func (wc *WorkflowCleaner) logError(ctx context.Context, err error) {
	// Returned errors are retries in batch processor
	// If we don't want a retry we just log here and return nil
	log.Error(ctx, "Error during workflow cleanup batch processing", err)
}
