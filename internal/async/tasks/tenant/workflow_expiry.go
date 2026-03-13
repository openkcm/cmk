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
}

func NewWorkflowExpiryProcessor(
	updater WorkflowExpiryUpdater,
	repo repo.Repo,
	opts ...async.TaskOption,
) async.TenantTaskHandler {
	w := &WorkflowExpiryProcessor{
		updater:   updater,
		repo:      repo,
		processor: async.NewBatchProcessor(repo),
	}
	for _, o := range opts {
		o(w)
	}

	return w
}

func (w *WorkflowExpiryProcessor) ProcessTask(ctx context.Context, task *asynq.Task) error {
	wfs, _, err := w.updater.GetWorkflows(ctx, manager.WorkflowFilter{})
	if err != nil {
		return err
	}
	for _, wf := range wfs {
		if wf.ExpiryDate == nil || time.Now().Before(*wf.ExpiryDate) {
			continue
		}

		err := w.expireWorkflow(ctx, wf.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *WorkflowExpiryProcessor) TenantQuery() *repo.Query {
	return repo.NewQuery()
}

func (w *WorkflowExpiryProcessor) TaskType() string {
	return config.TypeWorkflowExpire
}

func (w *WorkflowExpiryProcessor) FanOutFunc() async.FunOutFunc {
	return async.TenantFanOut
}

func (w *WorkflowExpiryProcessor) expireWorkflow(ctx context.Context, workflowID uuid.UUID) error {
	workflow, err := w.updater.TransitionWorkflow(ctx, workflowID, wfMechanism.TransitionExpire)
	if err != nil {
		return errs.Wrapf(err, "Failed to expire workflow")
	}
	log.Info(ctx, "Expired workflow", slog.String("workflow_id", workflow.ID.String()))

	return nil
}
