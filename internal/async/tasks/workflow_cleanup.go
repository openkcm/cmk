package tasks

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

type WorkflowRemoval interface {
	CleanupTerminalWorkflows(ctx context.Context) error
}

type WorkflowCleaner struct {
	workflowRemoval WorkflowRemoval
	repo            repo.Repo
	processor       *async.BatchProcessor
}

func NewWorkflowCleaner(
	workflowRemoval WorkflowRemoval,
	repo repo.Repo,
) *WorkflowCleaner {
	return &WorkflowCleaner{
		workflowRemoval: workflowRemoval,
		repo:            repo,
		processor:       async.NewBatchProcessor(repo),
	}
}

func (wc *WorkflowCleaner) ProcessTask(ctx context.Context, task *asynq.Task) error {
	log.Info(ctx, "Starting Workflow Cleanup Task")

	err := wc.processor.ProcessTenantsInBatch(
		ctx,
		task,
		repo.NewQuery(),
		func(ctx context.Context, tenant *model.Tenant, index int) error {
			cleanupErr := wc.workflowRemoval.CleanupTerminalWorkflows(ctx)
			if cleanupErr != nil {
				log.Error(ctx, "Running Workflow Cleanup", cleanupErr)
			}
			return nil
		},
	)
	if err != nil {
		log.Error(ctx, "Error during workflow cleanup batch processing", err)
		return errs.Wrap(ErrRunningTask, err)
	}

	return nil
}

func (wc *WorkflowCleaner) TaskType() string {
	return config.TypeWorkflowCleanup
}
