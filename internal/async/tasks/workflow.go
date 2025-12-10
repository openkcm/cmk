package tasks

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/log"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	wfMechanism "github.tools.sap/kms/cmk/internal/workflow"
	asyncUtils "github.tools.sap/kms/cmk/utils/async"
)

type WorkflowUpdater interface {
	AutoAssignApprovers(ctx context.Context, workflowID uuid.UUID) (*model.Workflow, error)
}

type WorkflowProcessor struct {
	updater WorkflowUpdater
	repo    repo.Repo
}

func NewWorkflowProcessor(
	updater WorkflowUpdater,
	repo repo.Repo,
) *WorkflowProcessor {
	return &WorkflowProcessor{
		updater: updater,
		repo:    repo,
	}
}

func (s *WorkflowProcessor) ProcessTask(ctx context.Context, task *asynq.Task) error {
	log.Info(ctx, "Started processing workflow auto assign task")

	payload, err := asyncUtils.ParseTaskPayload(task.Payload())
	if err != nil {
		log.Error(ctx, "Failed to parse task payload", err)
		return errs.Wrap(ErrRunningTask, err)
	}

	workflowID, err := uuid.ParseBytes(payload.Data)
	if err != nil {
		log.Error(ctx, "Failed to parse task payload data", err)
		return errs.Wrap(ErrRunningTask, err)
	}

	ctx = payload.InjectContext(ctx)

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

	workflow, err := s.updater.AutoAssignApprovers(ctx, workflowID)
	if err != nil {
		log.Error(ctx, "Failed to auto assign approvers", err)

		updateErr := s.putWorkflowInFailedState(ctx, workflowID, err.Error())
		if updateErr != nil {
			log.Error(ctx, "Failed to put workflow in failed state after error", updateErr)
		}

		return errs.Wrap(ErrRunningTask, err)
	}

	log.InjectTask(ctx, task)
	log.Info(ctx, "Auto assigned approvers to workflow",
		slog.String("workflow_id", workflow.ID.String()),
		slog.String("status", workflow.State))

	return nil
}

func (s *WorkflowProcessor) TaskType() string {
	return config.TypeWorkflowAutoAssign
}

func (s *WorkflowProcessor) putWorkflowInFailedState(
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
