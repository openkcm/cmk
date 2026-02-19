package eventprocessor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/openkcm/orbital"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients/registry"
	"github.com/openkcm/cmk/internal/clients/registry/systems"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/event-processor/proto"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type JobHandler interface {
	JobType() JobType
	ResolveTasks(ctx context.Context, job orbital.Job) ([]orbital.TaskInfo, error)
	Confirm(ctx context.Context, job orbital.Job) (orbital.JobConfirmResult, error)
	Terminate(ctx context.Context, job orbital.Job) error
}

type BaseSystemJobHandler struct {
	repo         repo.Repo
	cmkAuditor   *auditor.Auditor
	registry     registry.Service
	taskResolver *SystemTaskInfoResolver
}

func (h *BaseSystemJobHandler) ResolveTasks(
	ctx context.Context,
	job orbital.Job,
) ([]orbital.TaskInfo, error) {
	data, err := h.UnmarshalJobData(orbital.Job{Data: job.Data})
	if err != nil {
		return nil, err
	}

	var taskType proto.TaskType
	switch JobType(job.Type) {
	case JobTypeSystemLink:
		taskType = proto.TaskType_SYSTEM_LINK
	case JobTypeSystemUnlink:
		taskType = proto.TaskType_SYSTEM_UNLINK
	case JobTypeSystemSwitch:
		taskType = proto.TaskType_SYSTEM_SWITCH
	default:
		return nil, fmt.Errorf("unsupported job type: %s", job.Type)
	}

	taskInfo, err := h.taskResolver.GetTaskInfo(ctx, taskType, data)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve task info: %w", err)
	}

	return []orbital.TaskInfo{*taskInfo}, nil
}

func (h *BaseSystemJobHandler) Confirm(ctx context.Context, job orbital.Job) (orbital.JobConfirmResult, error) {
	data, err := h.UnmarshalJobData(job)
	if err != nil {
		return orbital.JobConfirmResult{
			IsCanceled:           false,
			CanceledErrorMessage: fmt.Sprintf("failed to unmarshal job data: %v", err),
		}, err
	}

	system, err := getSystemByID(ctx, h.repo, data.SystemID)
	if err != nil {
		return orbital.JobConfirmResult{
			IsCanceled:           false,
			CanceledErrorMessage: err.Error(),
		}, err
	}

	if system.Status != cmkapi.SystemStatusPROCESSING {
		return orbital.JobConfirmResult{
			IsCanceled:           true,
			CanceledErrorMessage: fmt.Sprintf("system status is in %v instead of processing", system.Status),
		}, nil
	}

	return orbital.JobConfirmResult{
		Done: true,
	}, nil
}

func (h *BaseSystemJobHandler) UnmarshalJobData(job orbital.Job) (SystemActionJobData, error) {
	var systemJobData SystemActionJobData

	err := json.Unmarshal(job.Data, &systemJobData)
	if err != nil {
		return SystemActionJobData{}, fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	return systemJobData, nil
}

func (h *BaseSystemJobHandler) UpdateSystem(ctx context.Context, system *model.System) error {
	ck := repo.NewCompositeKey().Where(repo.IDField, system.ID)
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)).UpdateAll(true)

	_, err := h.repo.Patch(ctx, system, *query)
	if err != nil {
		return fmt.Errorf("failed to update system %s status and keyConfigID: %w", system.ID, err)
	}

	return nil
}

func (h *BaseSystemJobHandler) CleanUpEvent(ctx context.Context, job orbital.Job) error {
	// Clean the event if it was successful as we no longer need to hold
	// previous state for cancel/retry actions
	_, err := h.repo.Delete(
		ctx,
		&model.Event{},
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(repo.NewCompositeKey().
			Where(repo.IdentifierField, job.ExternalID).
			Where(repo.TypeField, job.Type),
		)),
	)
	return err
}

func (h *BaseSystemJobHandler) Terminate(
	ctx context.Context,
	job orbital.Job,
	targetStatus cmkapi.SystemStatus,
	targetKeyClaim bool,
	auditFunc func(ctx context.Context, data SystemActionJobData, system model.System) error,
	getKeyConfigFunc func(ctx context.Context, repo repo.Repo, data SystemActionJobData) (*uuid.UUID, error),
) error {
	var status cmkapi.SystemStatus
	if job.Status == orbital.JobStatusDone {
		status = targetStatus
	} else {
		status = cmkapi.SystemStatusFAILED
	}

	data, err := h.UnmarshalJobData(job)
	if err != nil {
		return err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)
	system, err := getSystemByID(ctx, h.repo, data.SystemID)
	if err != nil {
		return err
	}
	if job.Status == orbital.JobStatusDone {
		err = h.CleanUpEvent(ctx, job)
		if err != nil {
			return fmt.Errorf("failed to clean up event for system %s: %w", system.ID, err)
		}
		err = auditFunc(ctx, data, *system)
		if err != nil {
			return fmt.Errorf("failed to send onboarding audit log for system %s: %w", system.ID, err)
		}

		err = h.SendL1KeyClaim(ctx, *system, data.TenantID, targetKeyClaim)
		if err != nil {
			return fmt.Errorf("failed to send L1 key claim for system %s: %w", system.ID, err)
		}
	}

	keyConfigID, err := getKeyConfigFunc(ctx, h.repo, data)
	err = h.UpdateSystem(ctx, &model.System{
		ID:                 system.ID,
		Status:             status,
		KeyConfigurationID: keyConfigID,
	})
	if err != nil {
		return fmt.Errorf("failed to update system %s: %w", system.ID, err)
	}

	return nil
}

func (h *BaseSystemJobHandler) SendL1KeyClaim(
	ctx context.Context,
	system model.System,
	tenant string,
	keyClaim bool,
) error {
	err := h.registry.System().ExtendedUpdateSystemL1KeyClaim(ctx, systems.SystemFilter{
		ExternalID: system.Identifier,
		Region:     system.Region,
		TenantID:   tenant,
	}, keyClaim)

	if errors.Is(err, systems.ErrKeyClaimAlreadyActive) && keyClaim ||
		errors.Is(err, systems.ErrKeyClaimAlreadyInactive) && !keyClaim {
		// If the key claim is already set to the desired state, we can ignore the error.
		return nil
	} else if err != nil {
		return errs.Wrap(ErrSettingKeyClaim, err)
	}

	return nil
}

type SystemLinkJobHandler struct {
	BaseSystemJobHandler
}

func NewSystemLinkJobHandler(
	repo repo.Repo,
	cmkAuditor *auditor.Auditor,
	registry registry.Service,
	taskResolver *SystemTaskInfoResolver,
) *SystemLinkJobHandler {
	return &SystemLinkJobHandler{
		BaseSystemJobHandler: BaseSystemJobHandler{
			repo:         repo,
			cmkAuditor:   cmkAuditor,
			registry:     registry,
			taskResolver: taskResolver,
		},
	}
}

func (h *SystemLinkJobHandler) JobType() JobType {
	return JobTypeSystemLink
}

func (h *SystemLinkJobHandler) Terminate(ctx context.Context, job orbital.Job) error {
	return h.BaseSystemJobHandler.Terminate(
		ctx, job,
		cmkapi.SystemStatusCONNECTED,
		false,
		func(ctx context.Context, data SystemActionJobData, system model.System) error {
			return h.cmkAuditor.SendCmkOnboardingAuditLog(ctx, data.KeyIDTo, system.Identifier)
		},
		func(ctx context.Context, repo repo.Repo, data SystemActionJobData) (*uuid.UUID, error) {
			key, err := getKeyByKeyID(ctx, repo, data.KeyIDTo)
			if err != nil {
				return nil, fmt.Errorf("failed to get key config ID for key %s: %w", data.KeyIDTo, err)
			}
			return &key.KeyConfigurationID, nil
		},
	)
}

type SystemUnlinkJobHandler struct {
	BaseSystemJobHandler
}

func NewSystemUnlinkJobHandler(
	repo repo.Repo,
	cmkAuditor *auditor.Auditor,
	registry registry.Service,
	taskResolver *SystemTaskInfoResolver,
) *SystemUnlinkJobHandler {
	return &SystemUnlinkJobHandler{
		BaseSystemJobHandler: BaseSystemJobHandler{
			repo:         repo,
			cmkAuditor:   cmkAuditor,
			registry:     registry,
			taskResolver: taskResolver,
		},
	}
}

func (h *SystemUnlinkJobHandler) JobType() JobType {
	return JobTypeSystemUnlink
}

func (h *SystemUnlinkJobHandler) Terminate(ctx context.Context, job orbital.Job) error {
	return h.BaseSystemJobHandler.Terminate(
		ctx, job,
		cmkapi.SystemStatusDISCONNECTED,
		false,
		func(ctx context.Context, data SystemActionJobData, system model.System) error {
			return h.cmkAuditor.SendCmkOffboardingAuditLog(ctx, data.KeyIDFrom, system.Identifier)
		},
		func(ctx context.Context, repo repo.Repo, data SystemActionJobData) (*uuid.UUID, error) {
			return nil, nil
		},
	)
}

type SystemSwitchJobHandler struct {
	BaseSystemJobHandler
}

func (h *SystemSwitchJobHandler) JobType() JobType {
	return JobTypeSystemSwitch
}

func NewSystemSwitchJobHandler(
	repo repo.Repo,
	cmkAuditor *auditor.Auditor,
	registry registry.Service,
	taskResolver *SystemTaskInfoResolver,
) *SystemSwitchJobHandler {
	return &SystemSwitchJobHandler{
		BaseSystemJobHandler: BaseSystemJobHandler{
			repo:         repo,
			cmkAuditor:   cmkAuditor,
			registry:     registry,
			taskResolver: taskResolver,
		},
	}
}

func (h *SystemSwitchJobHandler) Terminate(ctx context.Context, job orbital.Job) error {
	return h.BaseSystemJobHandler.Terminate(
		ctx, job,
		cmkapi.SystemStatusCONNECTED,
		true,
		func(ctx context.Context, data SystemActionJobData, system model.System) error {
			return h.cmkAuditor.SendCmkSwitchAuditLog(ctx, system.Identifier, data.KeyIDFrom, data.KeyIDTo)
		},
		func(ctx context.Context, repo repo.Repo, data SystemActionJobData) (*uuid.UUID, error) {
			key, err := getKeyByKeyID(ctx, repo, data.KeyIDTo)
			if err != nil {
				return nil, fmt.Errorf("failed to get key config ID for key %s: %w", data.KeyIDTo, err)
			}
			return &key.KeyConfigurationID, nil
		},
	)
}
