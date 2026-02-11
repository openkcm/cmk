package tasks

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	wfMechanism "github.com/openkcm/cmk/internal/workflow"
	ctxUtils "github.com/openkcm/cmk/utils/context"
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
	processor *BatchProcessor
}

func NewWorkflowExpiryProcessor(
	updater WorkflowExpiryUpdater,
	repo repo.Repo,
) *WorkflowExpiryProcessor {
	return &WorkflowExpiryProcessor{
		updater:   updater,
		repo:      repo,
		processor: NewBatchProcessor(repo),
	}
}

func (s *WorkflowExpiryProcessor) ProcessTask(ctx context.Context, task *asynq.Task) error {
	log.Info(ctx, "Starting Workflow Expiry Task")

	err := s.processor.ProcessTenantsInBatch(
		ctx,
		"Workflow Expiry",
		task,
		func(ctx context.Context, tenant *model.Tenant) error {
			log.Debug(ctx, "Processing expired workflows for tenant",
				slog.String("schemaName", tenant.SchemaName))

			wfs, _, getErr := s.updater.GetWorkflows(ctx, manager.WorkflowFilter{})
			if getErr != nil {
				return s.handleErrorTask(ctx, getErr)
			}
			for _, wf := range wfs {
				if wf.ExpiryDate == nil || time.Now().Before(*wf.ExpiryDate) {
					continue
				}

				expireErr := s.expireWorkflow(ctx, wf.ID)
				if expireErr != nil {
					return errs.Wrap(ErrRunningTask, expireErr)
				}
			}
			log.Debug(ctx, "Workflow expiry processing completed for tenant",
				slog.String("schemaName", tenant.SchemaName))
			return nil
		},
	)
	if err != nil {
		return s.handleErrorTenants(ctx, err)
	}

	return nil
}

func (s *WorkflowExpiryProcessor) TaskType() string {
	return config.TypeWorkflowExpire
}

func (s *WorkflowExpiryProcessor) expireWorkflow(ctx context.Context, workflowID uuid.UUID) error {
	ctx = ctxUtils.InjectSystemUser(ctx)
	workflow, err := s.updater.TransitionWorkflow(ctx, workflowID, wfMechanism.TransitionExpire)

	if err != nil {
		log.Error(ctx, "Failed to expire workflow", err)
		return err
	}
	log.Info(ctx, "Expired workflow", slog.String("workflow_id", workflow.ID.String()))

	return nil
}

func (s *WorkflowExpiryProcessor) handleErrorTenants(ctx context.Context, err error) error {
	log.Error(ctx, "Error during workflow expiry batch processing", err)
	return errs.Wrap(ErrRunningTask, err)
}

func (s *WorkflowExpiryProcessor) handleErrorTask(ctx context.Context, err error) error {
	log.Error(ctx, "Error running Workflow Expiry", err)
	return errs.Wrap(ErrRunningTask, err)
}
