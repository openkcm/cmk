package tasks

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"

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
	processor       *BatchProcessor
}

func NewWorkflowCleaner(
	workflowRemoval WorkflowRemoval,
	repo repo.Repo,
) *WorkflowCleaner {
	return &WorkflowCleaner{
		workflowRemoval: workflowRemoval,
		repo:            repo,
		processor:       NewBatchProcessor(repo),
	}
}

func (wc *WorkflowCleaner) ProcessTask(ctx context.Context, task *asynq.Task) error {
	log.Info(ctx, "Starting Workflow Cleanup Task")

	err := wc.processor.ProcessTenantsInBatch(ctx, "Workflow Cleanup", task,
		func(tenantCtx context.Context, tenant *model.Tenant, index int) error {
			log.Debug(tenantCtx, "Cleaning up expired workflows for tenant",
				slog.String("schemaName", tenant.SchemaName), slog.Int("index", index))

			cleanupErr := wc.workflowRemoval.CleanupTerminalWorkflows(tenantCtx)
			if cleanupErr != nil {
				log.Error(tenantCtx, "Running Workflow Cleanup", cleanupErr)
			}
			return nil
		})

	if err != nil {
		log.Error(ctx, "Error during workflow cleanup batch processing", err)
		return errs.Wrap(ErrRunningTask, err)
	}

	return nil
}

func (wc *WorkflowCleaner) TaskType() string {
	return config.TypeWorkflowCleanup
}
