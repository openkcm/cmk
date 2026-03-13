package tasks

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/repo"
)

type WorkflowRemoval interface {
	CleanupTerminalWorkflows(ctx context.Context) error
}

type WorkflowCleaner struct {
	workflowRemoval WorkflowRemoval
	repo            repo.Repo
	processor       *async.BatchProcessor
	fanout          bool
}

func NewWorkflowCleaner(
	workflowRemoval WorkflowRemoval,
	repo repo.Repo,
	opts ...async.TaskOption,
) *WorkflowCleaner {
	wc := &WorkflowCleaner{
		workflowRemoval: workflowRemoval,
		repo:            repo,
		processor:       async.NewBatchProcessor(repo),
	}

	for _, o := range opts {
		o(wc)
	}

	return wc
}

func (wc *WorkflowCleaner) Process(ctx context.Context, _ *asynq.Task) error {
	cleanupErr := wc.workflowRemoval.CleanupTerminalWorkflows(ctx)
	if cleanupErr != nil {
		log.Error(ctx, "Running Workflow Cleanup", cleanupErr)
	}
	return nil
}

func (wc *WorkflowCleaner) ProcessTask(ctx context.Context, task *asynq.Task) error {
	log.Info(ctx, "Starting Workflow Cleanup Task")

	err := wc.processor.ProcessTenantsInBatch(
		ctx,
		task,
		repo.NewQuery(),
		wc.Process,
	)
	if err != nil {
		log.Error(ctx, "Error during workflow cleanup batch processing", err)
		return errs.Wrap(ErrRunningTask, err)
	}

	return nil
}

func (wc *WorkflowCleaner) SetFanOut(client async.Client, opts ...asynq.Option) {
	wc.processor = async.NewBatchProcessor(wc.repo, async.WithFanOutTenants(client, opts...))
	wc.fanout = true
}

func (wc *WorkflowCleaner) IsFanOutEnabled() bool {
	return wc.fanout
}

func (wc *WorkflowCleaner) TaskType() string {
	return config.TypeWorkflowCleanup
}
