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

func (wc *WorkflowCleaner) ProcessTask(ctx context.Context, task *asynq.Task) error {
	log.Info(ctx, "Starting Workflow Cleanup Task")

	if async.IsChildTask(task) {
		return async.ProcessChildTask(ctx, task, wc.process)
	}

	err := wc.processor.ProcessTenantsInBatch(
		ctx,
		task,
		repo.NewQuery(),
		func(ctx context.Context, _ *model.Tenant) error {
			return wc.process(ctx)
		},
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

func (wc *WorkflowCleaner) process(ctx context.Context) error {
	cleanupErr := wc.workflowRemoval.CleanupTerminalWorkflows(ctx)
	if cleanupErr != nil {
		log.Error(ctx, "Running Workflow Cleanup", cleanupErr)
	}
	return nil
}
