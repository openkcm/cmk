package tasks

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	wfMechanism "github.com/openkcm/cmk/internal/workflow"
)

type WorkflowExpiryUpdater interface {
	GetWorkflows(ctx context.Context, params repo.QueryMapper) ([]*model.Workflow, int, error)
	TransitionWorkflow(
		ctx context.Context,
		workflowID uuid.UUID,
		transition wfMechanism.Transition,
	) (*model.Workflow, error)
}

type WorkflowExpiryProcessor struct {
	updater   WorkflowExpiryUpdater
	repo      repo.Repo
	processor *async.BatchProcessor
	fanout    bool
}

func NewWorkflowExpiryProcessor(
	updater WorkflowExpiryUpdater,
	repo repo.Repo,
	opts ...async.TaskOption,
) *WorkflowExpiryProcessor {
	w := &WorkflowExpiryProcessor{
		updater:   updater,
		repo:      repo,
		processor: async.NewBatchProcessor(repo),
	}
	for _, o := range opts {
		o(w)
	}

	log.Debug(context.Background(), "Created System Refresh Task")

	return w
}

func (w *WorkflowExpiryProcessor) ProcessTask(ctx context.Context, task *asynq.Task) error {
	log.Info(ctx, "Starting Workflow Expiry Task")

	if async.IsChildTask(task) {
		return async.ProcessChildTask(ctx, task, w.process)
	}

	err := w.processor.ProcessTenantsInBatch(
		ctx,
		task,
		repo.NewQuery(),
		func(ctx context.Context, _ *model.Tenant) error {
			return w.process(ctx)
		},
	)
	if err != nil {
		return w.handleErrorTenants(ctx, err)
	}

	return nil
}

func (w *WorkflowExpiryProcessor) process(ctx context.Context) error {
	wfs, _, getErr := w.updater.GetWorkflows(ctx, manager.WorkflowFilter{})
	if getErr != nil {
		log.Error(ctx, "Error running Workflow Expiry", getErr)
		return errs.Wrap(ErrRunningTask, getErr)
	}
	for _, wf := range wfs {
		if wf.ExpiryDate == nil || time.Now().Before(*wf.ExpiryDate) {
			continue
		}

		expireErr := w.expireWorkflow(ctx, wf.ID)
		if expireErr != nil {
			return errs.Wrap(ErrRunningTask, expireErr)
		}
	}
	return nil
}

func (w *WorkflowExpiryProcessor) TaskType() string {
	return config.TypeWorkflowExpire
}

func (w *WorkflowExpiryProcessor) expireWorkflow(ctx context.Context, workflowID uuid.UUID) error {
	workflow, err := w.updater.TransitionWorkflow(ctx, workflowID, wfMechanism.TransitionExpire)
	if err != nil {
		log.Error(ctx, "Failed to expire workflow", err)
		return err
	}
	log.Info(ctx, "Expired workflow", slog.String("workflow_id", workflow.ID.String()))

	return nil
}

func (w *WorkflowExpiryProcessor) SetFanOut(client async.Client) {
	w.processor = async.NewBatchProcessor(w.repo, async.WithFanOutTenants(client))
}

func (w *WorkflowExpiryProcessor) IsFanOutEnabled() bool {
	return w.fanout
}

func (w *WorkflowExpiryProcessor) handleErrorTenants(ctx context.Context, err error) error {
	log.Error(ctx, "Error during workflow expiry batch processing", err)
	return errs.Wrap(ErrRunningTask, err)
}
