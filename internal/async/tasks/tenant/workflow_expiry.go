package tasks

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	wfMechanism "github.com/openkcm/cmk/internal/workflow"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type WorkflowExpiryUpdater interface {
	GetWorkflows(ctx context.Context, params repo.QueryMapper) ([]*model.Workflow, int, error)
	TransitionWorkflow(
		ctx context.Context,
		workflowID uuid.UUID,
		transition wfMechanism.Transition,
	) (*model.Workflow, error)
	WorkflowCanExpire(ctx context.Context, workflow *model.Workflow) (bool, error)
}

type WorkflowExpiryProcessor struct {
	updater WorkflowExpiryUpdater
	repo    repo.Repo
}

func NewWorkflowExpiryProcessor(
	updater WorkflowExpiryUpdater,
	repo repo.Repo,
	opts ...async.TaskOption,
) async.TenantTaskHandler {
	w := &WorkflowExpiryProcessor{
		updater: updater,
		repo:    repo,
	}
	for _, o := range opts {
		o(w)
	}

	return w
}

func (w *WorkflowExpiryProcessor) ProcessTask(ctx context.Context, task *asynq.Task) error {
	ctx, err := cmkcontext.InjectInternalClientData(ctx,
		constants.InternalTaskWorkflowExpirationRole)
	if err != nil {
		w.logError(ctx, err)
		return nil
	}

	wfs, _, err := w.updater.GetWorkflows(ctx, manager.WorkflowFilter{})
	if err != nil {
		w.logError(ctx, err)
		return nil
	}

	for _, wf := range wfs {
		if wf.ExpiryDate == nil || time.Now().Before(*wf.ExpiryDate) {
			continue
		}

		canExpire, err := w.updater.WorkflowCanExpire(ctx, wf)
		if err != nil {
			log.Error(ctx, "Failed to check if workflow can expire", err,
				slog.String("workflow_id", wf.ID.String()))
			continue
		}

		if !canExpire {
			log.Debug(ctx, "Workflow cannot be expired from current state, skipping",
				slog.String("workflow_id", wf.ID.String()), slog.String("current_state", wf.State))
			continue
		}

		err = w.expireWorkflow(ctx, wf.ID)
		if err != nil {
			log.Error(ctx, "Failed to expire workflow", err, slog.String("workflow_id", wf.ID.String()))
			continue
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

func (w *WorkflowExpiryProcessor) FanOutFunc() async.FanOutFunc {
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

func (w *WorkflowExpiryProcessor) logError(ctx context.Context, err error) {
	// Returned errors are retries in batch processor
	// If we don't want a retry we just log here and return nil
	log.Error(ctx, "Error during workflow expiry batch processing", err)
}
