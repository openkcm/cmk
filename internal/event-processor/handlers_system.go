package eventprocessor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/openkcm/orbital"

	mappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients/registry"
	"github.com/openkcm/cmk/internal/clients/registry/systems"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/event-processor/proto"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type BaseSystemJobHandler struct {
	repo         repo.Repo
	cmkAuditor   *auditor.Auditor
	registry     registry.Service
	taskResolver *SystemTaskInfoResolver

	keyClaimFunc     func(ctx context.Context, system model.System, tenant string) error
	auditFunc        func(ctx context.Context, data SystemActionJobData, system model.System) error
	getKeyConfigFunc func(ctx context.Context, repo repo.Repo, data SystemActionJobData) (*uuid.UUID, error)
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
	case JobTypeSystemSwitch, JobTypeSystemSwitchNewPK:
		taskType = proto.TaskType_SYSTEM_SWITCH
	default:
		return nil, errs.Wrapf(ErrInvalidJobType, job.Type)
	}

	taskInfo, err := h.taskResolver.GetTaskInfo(ctx, taskType, data)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve task info: %w", err)
	}

	return []orbital.TaskInfo{*taskInfo}, nil
}

func (h *BaseSystemJobHandler) HandleJobConfirm(
	ctx context.Context,
	job orbital.Job,
) (orbital.JobConfirmerResult, error) {
	data, err := h.UnmarshalJobData(job)
	if err != nil {
		return orbital.CancelJobConfirmer(fmt.Sprintf("failed to unmarshal job data: %v", err)), err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)
	system, err := getSystemByID(ctx, h.repo, data.SystemID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return orbital.CancelJobConfirmer(fmt.Sprintf("system with ID %s not found", data.SystemID)), nil
		}
		// For any other error, we should return the error to trigger a retry, as it could be a transient issue.
		return nil, fmt.Errorf("failed to get system by ID %s: %w", data.SystemID, err)
	}

	if system.Status != cmkapi.SystemStatusPROCESSING {
		return orbital.CancelJobConfirmer(fmt.Sprintf("system %s is not in processing status", system.ID)), nil
	}

	return orbital.CompleteJobConfirmer(), nil
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

func (h *BaseSystemJobHandler) TerminateFailedOrCanceled(
	ctx context.Context,
	job orbital.Job,
) error {
	log.InjectSystemEvent(ctx, job.Type)

	data, err := h.UnmarshalJobData(job)
	if err != nil {
		return err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)
	system, err := getSystemByID(ctx, h.repo, data.SystemID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			log.Warn(ctx, "System not found when handling job termination, skipping system update",
				slog.String("systemID", data.SystemID))
			return nil
		}
		return err
	}

	system.Status = cmkapi.SystemStatusFAILED
	err = h.UpdateSystem(ctx, system)

	if err != nil {
		return fmt.Errorf("failed to update system %s: %w", system.ID, err)
	}

	return nil
}

//nolint:funlen,cyclop
func (h *BaseSystemJobHandler) TerminateDone(
	ctx context.Context,
	job orbital.Job,
	targetStatus cmkapi.SystemStatus,
) error {
	log.InjectSystemEvent(ctx, job.Type)

	data, err := h.UnmarshalJobData(job)
	if err != nil {
		return err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)
	system, err := getSystemByID(ctx, h.repo, data.SystemID)
	if err != nil {
		return err
	}

	err = h.CleanUpEvent(ctx, job)
	if err != nil {
		return fmt.Errorf("failed to clean up event for system %s: %w", system.ID, err)
	}

	if h.auditFunc != nil {
		err = h.auditFunc(ctx, data, *system)
		if err != nil {
			return fmt.Errorf("failed to send onboarding audit log for system %s: %w", system.ID, err)
		}
	}

	if h.keyClaimFunc != nil {
		err = h.keyClaimFunc(ctx, *system, data.TenantID)
		if err != nil {
			return fmt.Errorf("failed to update key claim for system %s: %w", system.ID, err)
		}
	}

	// This should be moved out of here on tenant decommission refactor
	if data.Trigger == constants.SystemActionDecommission {
		_, err = h.registry.Mapping().UnmapSystemFromTenant(ctx, &mappingv1.UnmapSystemFromTenantRequest{
			ExternalId: system.Identifier,
			Type:       strings.ToLower(system.Type),
			TenantId:   data.TenantID,
		})

		if err != nil {
			return fmt.Errorf("failed to unmap system from tenant: %w", err)
		}
	}

	if h.getKeyConfigFunc == nil {
		log.Warn(ctx, "getKeyConfigFunc is not set, cannot update system with key configuration ID")
		return nil
	}

	keyConfigID, err := h.getKeyConfigFunc(ctx, h.repo, data)
	if err != nil {
		return fmt.Errorf("failed to get key config ID: %w", err)
	}

	system.Status = targetStatus
	system.KeyConfigurationID = keyConfigID
	err = h.UpdateSystem(ctx, system)
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
	handler := &SystemLinkJobHandler{}
	handler.BaseSystemJobHandler = BaseSystemJobHandler{
		repo:             repo,
		cmkAuditor:       cmkAuditor,
		registry:         registry,
		taskResolver:     taskResolver,
		auditFunc:        handler.auditFunc,
		getKeyConfigFunc: handler.getKeyConfigFunc,
		keyClaimFunc:     handler.keyClaimFunc,
	}

	return handler
}

func (h *SystemLinkJobHandler) HandleJobDoneEvent(ctx context.Context, job orbital.Job) error {
	return h.TerminateDone(ctx, job, cmkapi.SystemStatusCONNECTED)
}

func (h *BaseSystemJobHandler) HandleJobFailedEvent(ctx context.Context, job orbital.Job) error {
	return h.TerminateFailedOrCanceled(ctx, job)
}

func (h *BaseSystemJobHandler) HandleJobCanceledEvent(ctx context.Context, job orbital.Job) error {
	return h.TerminateFailedOrCanceled(ctx, job)
}

func (h *SystemLinkJobHandler) keyClaimFunc(ctx context.Context, system model.System, tenant string) error {
	return h.SendL1KeyClaim(ctx, system, tenant, true)
}

func (h *SystemLinkJobHandler) auditFunc(
	ctx context.Context,
	data SystemActionJobData,
	system model.System,
) error {
	return h.cmkAuditor.SendCmkOnboardingAuditLog(ctx, system.Identifier, data.KeyIDTo)
}

func (h *SystemLinkJobHandler) getKeyConfigFunc(
	ctx context.Context,
	repo repo.Repo,
	data SystemActionJobData,
) (*uuid.UUID, error) {
	key, err := getKeyByKeyID(ctx, repo, data.KeyIDTo)
	if err != nil {
		return nil, fmt.Errorf("failed to get key config ID for key %s: %w", data.KeyIDTo, err)
	}

	return &key.KeyConfigurationID, nil
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
	handler := &SystemUnlinkJobHandler{}
	handler.BaseSystemJobHandler = BaseSystemJobHandler{
		repo:             repo,
		cmkAuditor:       cmkAuditor,
		registry:         registry,
		taskResolver:     taskResolver,
		auditFunc:        handler.auditFunc,
		getKeyConfigFunc: handler.getKeyConfigFunc,
		keyClaimFunc:     handler.keyClaimFunc,
	}

	return handler
}

func (h *SystemUnlinkJobHandler) HandleJobDoneEvent(ctx context.Context, job orbital.Job) error {
	return h.TerminateDone(ctx, job, cmkapi.SystemStatusDISCONNECTED)
}

func (h *SystemUnlinkJobHandler) HandleJobFailedEvent(ctx context.Context, job orbital.Job) error {
	return h.TerminateFailedOrCanceled(ctx, job)
}

func (h *SystemUnlinkJobHandler) HandleJobCanceledEvent(ctx context.Context, job orbital.Job) error {
	return h.TerminateFailedOrCanceled(ctx, job)
}

func (h *SystemUnlinkJobHandler) auditFunc(
	ctx context.Context,
	data SystemActionJobData,
	system model.System,
) error {
	return h.cmkAuditor.SendCmkOffboardingAuditLog(ctx, system.Identifier, data.KeyIDFrom)
}

func (h *SystemUnlinkJobHandler) getKeyConfigFunc(
	_ context.Context,
	_ repo.Repo,
	_ SystemActionJobData,
) (*uuid.UUID, error) {
	return nil, nil //nolint:nilnil
}

func (h *SystemUnlinkJobHandler) keyClaimFunc(ctx context.Context, system model.System, tenant string) error {
	return h.SendL1KeyClaim(ctx, system, tenant, false)
}

type SystemSwitchJobHandler struct {
	BaseSystemJobHandler
}

func NewSystemSwitchJobHandler(
	repo repo.Repo,
	cmkAuditor *auditor.Auditor,
	registry registry.Service,
	taskResolver *SystemTaskInfoResolver,
) *SystemSwitchJobHandler {
	handler := &SystemSwitchJobHandler{}
	handler.BaseSystemJobHandler = BaseSystemJobHandler{
		repo:             repo,
		cmkAuditor:       cmkAuditor,
		registry:         registry,
		taskResolver:     taskResolver,
		auditFunc:        handler.auditFunc,
		getKeyConfigFunc: handler.getKeyConfigFunc,
		keyClaimFunc:     handler.keyClaimFunc,
	}

	return handler
}

func (h *SystemSwitchJobHandler) HandleJobDoneEvent(ctx context.Context, job orbital.Job) error {
	return h.TerminateDone(ctx, job, cmkapi.SystemStatusCONNECTED)
}

func (h *SystemSwitchJobHandler) HandleJobFailedEvent(ctx context.Context, job orbital.Job) error {
	return h.TerminateFailedOrCanceled(ctx, job)
}

func (h *SystemSwitchJobHandler) HandleJobCanceledEvent(ctx context.Context, job orbital.Job) error {
	return h.TerminateFailedOrCanceled(ctx, job)
}

func (h *SystemSwitchJobHandler) auditFunc(
	ctx context.Context,
	data SystemActionJobData,
	system model.System,
) error {
	return h.cmkAuditor.SendCmkSwitchAuditLog(ctx, system.Identifier, data.KeyIDFrom, data.KeyIDTo)
}

func (h *SystemSwitchJobHandler) getKeyConfigFunc(
	ctx context.Context,
	repo repo.Repo,
	data SystemActionJobData,
) (*uuid.UUID, error) {
	key, err := getKeyByKeyID(ctx, repo, data.KeyIDTo)
	if err != nil {
		return nil, fmt.Errorf("failed to get key config ID for key %s: %w", data.KeyIDTo, err)
	}

	return &key.KeyConfigurationID, nil
}

func (h *SystemSwitchJobHandler) keyClaimFunc(_ context.Context, _ model.System, _ string) error {
	return nil
}
