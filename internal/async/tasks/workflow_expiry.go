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
)

type WorkflowExpiryUpdater interface {
	GetWorkflows(ctx context.Context, params repo.QueryMapper) ([]*model.Workflow, int, error)
	TransitionWorkflow(ctx context.Context, userID uuid.UUID,
		workflowID uuid.UUID, transition wfMechanism.Transition) (*model.Workflow, error)
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

	err := s.processor.ProcessTenantsInBatch(ctx, "Workflow Expiry", task,
		func(tenantCtx context.Context, tenant *model.Tenant, index int) error {
			log.Debug(tenantCtx, "Processing expired workflows for tenant",
				slog.String("schemaName", tenant.SchemaName), slog.Int("index", index))

			wfs, _, getErr := s.updater.GetWorkflows(tenantCtx, manager.WorkflowFilter{})
			if getErr != nil {
				return s.handleErrorTask(tenantCtx, getErr)
			}
			for _, wf := range wfs {
				if wf.ExpiryDate == nil || time.Now().Before(*wf.ExpiryDate) {
					continue
				}

				expireErr := s.expireWorkflow(tenantCtx, wf.ID)
				if expireErr != nil {
					return errs.Wrap(ErrRunningTask, expireErr)
				}
			}
			log.Debug(tenantCtx, "Workflow expiry processing completed for tenant",
				slog.String("schemaName", tenant.SchemaName))
			return nil
		})

	if err != nil {
		return s.handleErrorTenants(ctx, err)
	}

	return nil
}

func (s *WorkflowExpiryProcessor) TaskType() string {
	return config.TypeWorkflowExpire
}

func (s *WorkflowExpiryProcessor) expireWorkflow(ctx context.Context, workflowID uuid.UUID) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(ctx, "Panic occurred while processing workflow auto assign task", nil,
				slog.Any("panic", r),
			)

			updateErr := s.putWorkflowInFailedState(ctx, workflowID, "internal error when assigning approvers")
			if updateErr != nil {
				log.Error(ctx, "Failed to put workflow in failed state after panic", updateErr)
			}
		}
	}()

	workflow, err := s.updater.TransitionWorkflow(ctx, wfMechanism.SystemUserUUID, workflowID,
		wfMechanism.TransitionExpire)
	if err != nil {
		log.Error(ctx, "Failed to auto assign approvers", err)

		updateErr := s.putWorkflowInFailedState(ctx, workflowID, err.Error())
		if updateErr != nil {
			log.Error(ctx, "Failed to put workflow in failed state after error", updateErr)
		}

		return err
	}
	log.Info(ctx, "Expiredworkflow",
		slog.String("workflow_id", workflow.ID.String()))

	return nil
}

func (s *WorkflowExpiryProcessor) putWorkflowInFailedState(
	ctx context.Context,
	workflowID uuid.UUID,
	failureReason string,
) error {
	workflow := &model.Workflow{
		ID:            workflowID,
		State:         wfMechanism.StateFailed.String(),
		FailureReason: failureReason,
	}

	_, err := s.repo.Patch(ctx, workflow, *repo.NewQuery())

	return err
}

func (s *WorkflowExpiryProcessor) handleErrorTenants(ctx context.Context, err error) error {
	log.Error(ctx, "Error during workflow expiry batch processing", err)
	return errs.Wrap(ErrRunningTask, err)
}

func (s *WorkflowExpiryProcessor) handleErrorTask(ctx context.Context, err error) error {
	log.Error(ctx, "Running Cert Refresh", err)
	return errs.Wrap(ErrRunningTask, err)
}
