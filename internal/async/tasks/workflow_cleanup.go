package tasks

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
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

	err := wc.processor.ProcessTenantsInBatchWithOptions(
		ctx,
		"Workflow Cleanup",
		task,
		repo.NewQuery(),
		repo.BatchProcessOptions{IgnoreFailMode: true},
		func(ctx context.Context, tenant *model.Tenant, index int) error {
			log.Debug(ctx, "Cleaning up expired workflows for tenant",
				slog.String("schemaName", tenant.SchemaName), slog.Int("index", index))
			ctx, err := cmkcontext.InjectInternalClientData(ctx,
				constants.InternalTaskWorkflowCleanupRole)
			if err != nil {
				return wc.handleErrorTask(ctx, err)
			}

			cleanupErr := wc.workflowRemoval.CleanupTerminalWorkflows(ctx)
			if cleanupErr != nil {
				return wc.handleErrorTask(ctx, cleanupErr)
			}
			return nil
		},
	)
	if err != nil {
		return wc.handleErrorTenants(ctx, err)
	}

	return nil
}

func (wc *WorkflowCleaner) TaskType() string {
	return config.TypeWorkflowCleanup
}

func (t *WorkflowCleaner) handleErrorTenants(ctx context.Context, err error) error {
	log.Error(ctx, "Error during workflow cleanup sync batch processing", err)
	return errs.Wrap(ErrRunningTask, err)
}

func (t *WorkflowCleaner) handleErrorTask(ctx context.Context, err error) error {
	log.Error(ctx, "Running workflow cleanup", err)
	return errs.Wrap(ErrRunningTask, err)
}
