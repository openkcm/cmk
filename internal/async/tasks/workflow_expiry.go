package tasks

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/log"
	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	wfMechanism "github.tools.sap/kms/cmk/internal/workflow"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

type WorkflowExpiryUpdater interface {
	GetWorkflows(ctx context.Context, params repo.QueryMapper) ([]*model.Workflow, int, error)
	TransitionWorkflow(ctx context.Context, userID uuid.UUID,
		workflowID uuid.UUID, transition wfMechanism.Transition) (*model.Workflow, error)
}

type WorkflowExpiryProcessor struct {
	updater WorkflowExpiryUpdater
	repo    repo.Repo
}

func NewWorkflowExpiryProcessor(
	updater WorkflowExpiryUpdater,
	repo repo.Repo,
) *WorkflowExpiryProcessor {
	return &WorkflowExpiryProcessor{
		updater: updater,
		repo:    repo,
	}
}

func (s *WorkflowExpiryProcessor) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	log.Info(ctx, "Started processing workflow expiry auto assign task")

	var tenants []*model.Tenant

	_, err := s.repo.List(ctx, model.Tenant{}, &tenants, *repo.NewQuery())
	if err != nil {
		return s.handleErrorTenants(ctx, err)
	}

	for _, tenant := range tenants {
		ctx := log.InjectTenant(cmkcontext.CreateTenantContext(ctx, tenant.ID), tenant)

		wfs, _, err := s.updater.GetWorkflows(ctx, manager.WorkflowFilter{})
		if err != nil {
			return s.handleErrorTask(ctx, err)
		}
		for _, wf := range wfs {
			if time.Now().Before(wf.ExpiryDate) {
				continue
			}

			err = s.expireWorkflow(ctx, wf.ID)
			if err != nil {
				return errs.Wrap(ErrRunningTask, err)
			}
		}
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
	log.Error(ctx, "Getting Tenants on Cert Refresh", err)
	return errs.Wrap(ErrRunningTask, err)
}

func (s *WorkflowExpiryProcessor) handleErrorTask(ctx context.Context, err error) error {
	log.Error(ctx, "Running Cert Refresh", err)
	return errs.Wrap(ErrRunningTask, err)
}
