package eventprocessor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/openkcm/orbital"

	mappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients/registry"
	"github.com/openkcm/cmk/internal/clients/registry/systems"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type SystemLinkJobHandler struct {
	repo           repo.Repo
	registry       registry.Service
	cmkAuditor     *auditor.Auditor
	orbitalManager *orbital.Manager
	taskResolver   *SystemTaskInfoResolver
}

func NewSystemLinkJobHandler(
	repo repo.Repo,
	registry registry.Service,
	cmkAuditor *auditor.Auditor,
	orbitalManager *orbital.Manager,
	taskResolver *SystemTaskInfoResolver,
) *SystemLinkJobHandler {
	return &SystemLinkJobHandler{
		repo:           repo,
		registry:       registry,
		cmkAuditor:     cmkAuditor,
		orbitalManager: orbitalManager,
		taskResolver:   taskResolver,
	}
}

func (h *SystemLinkJobHandler) ResolveTasks(
	ctx context.Context,
	job orbital.Job,
) ([]orbital.TaskInfo, error) {
	return h.taskResolver.Resolve(ctx, job)
}

func (h *SystemLinkJobHandler) HandleJobConfirm(
	ctx context.Context,
	job orbital.Job,
) (orbital.JobConfirmerResult, error) {
	return handleSystemJobConfirm(ctx, h.repo, job)
}

func (h *SystemLinkJobHandler) HandleJobDoneEvent(ctx context.Context, job orbital.Job) error {
	log.InjectSystemEvent(ctx, job.Type)

	data, err := unmarshalSystemJobData(job)
	if err != nil {
		return err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)
	system, err := getSystemByID(ctx, h.repo, data.SystemID)
	if err != nil {
		return err
	}

	err = h.cmkAuditor.SendCmkOnboardingAuditLog(ctx, system.Identifier, data.KeyIDTo)
	if err != nil {
		return fmt.Errorf("failed to send onboarding audit log for system %s: %w", system.ID, err)
	}

	err = sendL1KeyClaim(ctx, *system, h.registry, data.TenantID, true)
	if err != nil {
		return fmt.Errorf("failed to update key claim for system %s: %w", system.ID, err)
	}

	key, err := getKeyByKeyID(ctx, h.repo, data.KeyIDTo)
	if err != nil {
		return fmt.Errorf("failed to get key config ID for key %s: %w", data.KeyIDTo, err)
	}

	system.Status = cmkapi.SystemStatusCONNECTED
	system.KeyConfigurationID = &key.KeyConfigurationID
	err = updateSystem(ctx, h.repo, system)
	if err != nil {
		return fmt.Errorf("failed to update system %s: %w", system.ID, err)
	}

	err = cleanUpEvent(ctx, h.repo, job)
	if err != nil {
		return fmt.Errorf("failed to clean up event for system %s: %w", system.ID, err)
	}

	return nil
}

func (h *SystemLinkJobHandler) HandleJobFailedEvent(ctx context.Context, job orbital.Job) error {
	return terminateFailedSystemJob(ctx, h.orbitalManager, h.repo, job)
}

func (h *SystemLinkJobHandler) HandleJobCanceledEvent(ctx context.Context, job orbital.Job) error {
	return terminateCanceledSystemJob(ctx, h.repo, job)
}

type SystemUnlinkJobHandler struct {
	repo           repo.Repo
	registry       registry.Service
	cmkAuditor     *auditor.Auditor
	orbitalManager *orbital.Manager
	taskResolver   *SystemTaskInfoResolver
}

func NewSystemUnlinkJobHandler(
	repo repo.Repo,
	registry registry.Service,
	cmkAuditor *auditor.Auditor,
	orbitalManager *orbital.Manager,
	taskResolver *SystemTaskInfoResolver,
) *SystemUnlinkJobHandler {
	return &SystemUnlinkJobHandler{
		repo:           repo,
		registry:       registry,
		cmkAuditor:     cmkAuditor,
		orbitalManager: orbitalManager,
		taskResolver:   taskResolver,
	}
}

func (h *SystemUnlinkJobHandler) ResolveTasks(
	ctx context.Context,
	job orbital.Job,
) ([]orbital.TaskInfo, error) {
	return h.taskResolver.Resolve(ctx, job)
}

func (h *SystemUnlinkJobHandler) HandleJobConfirm(
	ctx context.Context,
	job orbital.Job,
) (orbital.JobConfirmerResult, error) {
	return handleSystemJobConfirm(ctx, h.repo, job)
}

func (h *SystemUnlinkJobHandler) HandleJobDoneEvent(ctx context.Context, job orbital.Job) error {
	log.InjectSystemEvent(ctx, job.Type)

	data, err := unmarshalSystemJobData(job)
	if err != nil {
		return err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)
	system, err := getSystemByID(ctx, h.repo, data.SystemID)
	if err != nil {
		return err
	}

	err = h.cmkAuditor.SendCmkOffboardingAuditLog(ctx, system.Identifier, data.KeyIDFrom)
	if err != nil {
		return fmt.Errorf("failed to send offboarding audit log for system %s: %w", system.ID, err)
	}

	err = sendL1KeyClaim(ctx, *system, h.registry, data.TenantID, false)
	if err != nil {
		return fmt.Errorf("failed to update key claim for system %s: %w", system.ID, err)
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

	system.Status = cmkapi.SystemStatusDISCONNECTED
	system.KeyConfigurationID = nil
	err = updateSystem(ctx, h.repo, system)
	if err != nil {
		return fmt.Errorf("failed to update system %s: %w", system.ID, err)
	}

	err = cleanUpEvent(ctx, h.repo, job)
	if err != nil {
		return fmt.Errorf("failed to clean up event for system %s: %w", system.ID, err)
	}

	return nil
}

func (h *SystemUnlinkJobHandler) HandleJobFailedEvent(ctx context.Context, job orbital.Job) error {
	return terminateFailedSystemJob(ctx, h.orbitalManager, h.repo, job)
}

func (h *SystemUnlinkJobHandler) HandleJobCanceledEvent(ctx context.Context, job orbital.Job) error {
	return terminateCanceledSystemJob(ctx, h.repo, job)
}

type SystemSwitchJobHandler struct {
	repo           repo.Repo
	registry       registry.Service
	cmkAuditor     *auditor.Auditor
	orbitalManager *orbital.Manager
	taskResolver   *SystemTaskInfoResolver
}

func NewSystemSwitchJobHandler(
	repo repo.Repo,
	registry registry.Service,
	cmkAuditor *auditor.Auditor,
	orbitalManager *orbital.Manager,
	taskResolver *SystemTaskInfoResolver,
) *SystemSwitchJobHandler {
	return &SystemSwitchJobHandler{
		repo:           repo,
		registry:       registry,
		cmkAuditor:     cmkAuditor,
		orbitalManager: orbitalManager,
		taskResolver:   taskResolver,
	}
}

func (h *SystemSwitchJobHandler) ResolveTasks(
	ctx context.Context,
	job orbital.Job,
) ([]orbital.TaskInfo, error) {
	return h.taskResolver.Resolve(ctx, job)
}

func (h *SystemSwitchJobHandler) HandleJobConfirm(
	ctx context.Context,
	job orbital.Job,
) (orbital.JobConfirmerResult, error) {
	return handleSystemJobConfirm(ctx, h.repo, job)
}

func (h *SystemSwitchJobHandler) HandleJobDoneEvent(ctx context.Context, job orbital.Job) error {
	log.InjectSystemEvent(ctx, job.Type)

	data, err := unmarshalSystemJobData(job)
	if err != nil {
		return err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)
	system, err := getSystemByID(ctx, h.repo, data.SystemID)
	if err != nil {
		return err
	}

	err = h.cmkAuditor.SendCmkSwitchAuditLog(ctx, system.Identifier, data.KeyIDFrom, data.KeyIDTo)
	if err != nil {
		return fmt.Errorf("failed to send rotation audit log for system %s: %w", system.ID, err)
	}

	key, err := getKeyByKeyID(ctx, h.repo, data.KeyIDTo)
	if err != nil {
		return fmt.Errorf("failed to get key config ID for key %s: %w", data.KeyIDTo, err)
	}

	system.KeyConfigurationID = &key.KeyConfigurationID
	err = updateSystem(ctx, h.repo, system)
	if err != nil {
		return fmt.Errorf("failed to update system %s: %w", system.ID, err)
	}

	err = cleanUpEvent(ctx, h.repo, job)
	if err != nil {
		return fmt.Errorf("failed to clean up event for system %s: %w", system.ID, err)
	}

	return nil
}

func (h *SystemSwitchJobHandler) HandleJobFailedEvent(ctx context.Context, job orbital.Job) error {
	return terminateFailedSystemJob(ctx, h.orbitalManager, h.repo, job)
}

func (h *SystemSwitchJobHandler) HandleJobCanceledEvent(ctx context.Context, job orbital.Job) error {
	return terminateCanceledSystemJob(ctx, h.repo, job)
}

func handleSystemJobConfirm(
	ctx context.Context,
	r repo.Repo,
	job orbital.Job,
) (orbital.JobConfirmerResult, error) {
	data, err := unmarshalSystemJobData(job)
	if err != nil {
		return orbital.CancelJobConfirmer(fmt.Sprintf("failed to unmarshal job data: %v", err)), err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)
	system, err := getSystemByID(ctx, r, data.SystemID)
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

func terminateFailedSystemJob(
	ctx context.Context,
	orbitalManager *orbital.Manager,
	r repo.Repo,
	job orbital.Job,
) error {
	system, err := getSystemFromJob(ctx, r, job)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			log.Warn(ctx, "System not found when handling job termination, skipping system update",
				slog.String("systemID", system.Identifier))
			return nil
		}
		return err
	}

	errorMessage, err := mergeOrbitalTaskErrors(ctx, orbitalManager, job)
	if err != nil {
		log.Error(ctx, "Failed to merge orbital task errors", err, slog.String("jobID", job.ID.String()))
		errorMessage = job.ErrorMessage // Fall back to the job error message if we fail to get task errors
	}

	// Attempt to get task error messages from orbital to provide more context on the failure
	err = updateEventError(ctx, r, job.ExternalID, errorMessage)
	if err != nil {
		return err
	}

	system.Status = cmkapi.SystemStatusFAILED
	err = updateSystem(ctx, r, system)

	if err != nil {
		return fmt.Errorf("failed to update system %s: %w", system.ID, err)
	}

	return nil
}

func terminateCanceledSystemJob(
	ctx context.Context,
	r repo.Repo,
	job orbital.Job,
) error {
	system, err := getSystemFromJob(ctx, r, job)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			log.Warn(ctx, "System not found when handling job termination, skipping system update",
				slog.String("systemID", system.Identifier))
			return nil
		}
		return err
	}

	// Attempt to get task error messages from orbital to provide more context on the cancellation
	err = updateEventError(ctx, r, job.ExternalID, job.ErrorMessage)
	if err != nil {
		return err
	}

	system.Status = cmkapi.SystemStatusFAILED
	err = updateSystem(ctx, r, system)

	if err != nil {
		return fmt.Errorf("failed to update system %s: %w", system.ID, err)
	}

	return nil
}

func sendL1KeyClaim(
	ctx context.Context,
	system model.System,
	registry registry.Service,
	tenant string,
	keyClaim bool,
) error {
	err := registry.System().ExtendedUpdateSystemL1KeyClaim(ctx, systems.SystemFilter{
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

func updateEventError(ctx context.Context, r repo.Repo, identifier string, errorMessage string) error {
	orbitalErr := ParseOrbitalError(errorMessage)
	event := &model.Event{Identifier: identifier}
	event.ErrorCode = orbitalErr.Code
	event.ErrorMessage = orbitalErr.Message

	_, err := r.Patch(ctx, event, *repo.NewQuery())
	return err
}

func getSystemFromJob(ctx context.Context, r repo.Repo, job orbital.Job) (*model.System, error) {
	log.InjectSystemEvent(ctx, job.Type)

	data, err := unmarshalSystemJobData(job)
	if err != nil {
		return nil, err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)
	system, err := getSystemByID(ctx, r, data.SystemID)
	if err != nil {
		return nil, err
	}

	return system, nil
}

func updateSystem(ctx context.Context, r repo.Repo, system *model.System) error {
	ck := repo.NewCompositeKey().Where(repo.IDField, system.ID)
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)).UpdateAll(true)

	_, err := r.Patch(ctx, system, *query)
	if err != nil {
		return fmt.Errorf("failed to update system %s upon job termination: %w", system.ID, err)
	}

	return nil
}

func cleanUpEvent(ctx context.Context, r repo.Repo, job orbital.Job) error {
	// Clean the event if it was successful as we no longer need to hold
	// previous state for cancel/retry actions
	_, err := r.Delete(
		ctx,
		&model.Event{},
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(repo.NewCompositeKey().
			Where(repo.IdentifierField, job.ExternalID).
			Where(repo.TypeField, job.Type),
		)),
	)
	return err
}

func mergeOrbitalTaskErrors(
	ctx context.Context,
	orbitalManager *orbital.Manager,
	job orbital.Job,
) (string, error) {
	tasks, err := orbitalManager.ListTasks(ctx, orbital.ListTasksQuery{
		JobID:  job.ID,
		Status: orbital.TaskStatusFailed,
	})

	if err != nil {
		return "", err
	}

	taskErrors := make([]string, 0, len(tasks))
	for _, t := range tasks {
		taskErrors = append(taskErrors, t.ErrorMessage)
	}
	message := strings.Join(taskErrors, ":")

	return message, nil
}

// SystemKeyRotateJobHandler handles SYSTEM_KEY_ROTATE events.
// This event notifies Kernel Service about key material rotation.
type SystemKeyRotateJobHandler struct {
	repo           repo.Repo
	cmkAuditor     *auditor.Auditor
	orbitalManager *orbital.Manager
	taskResolver   *SystemTaskInfoResolver
}

func NewSystemKeyRotateJobHandler(
	repo repo.Repo,
	cmkAuditor *auditor.Auditor,
	orbitalManager *orbital.Manager,
	taskResolver *SystemTaskInfoResolver,
) *SystemKeyRotateJobHandler {
	return &SystemKeyRotateJobHandler{
		repo:           repo,
		cmkAuditor:     cmkAuditor,
		orbitalManager: orbitalManager,
		taskResolver:   taskResolver,
	}
}

func (h *SystemKeyRotateJobHandler) ResolveTasks(
	ctx context.Context,
	job orbital.Job,
) ([]orbital.TaskInfo, error) {
	return h.taskResolver.Resolve(ctx, job)
}

func (h *SystemKeyRotateJobHandler) HandleJobConfirm(
	ctx context.Context,
	job orbital.Job,
) (orbital.JobConfirmerResult, error) {
	// SYSTEM_KEY_ROTATE requires system to be in PROCESSING state
	// Same as SYSTEM_SWITCH - both involve re-encryption in Kernel Service
	return handleSystemJobConfirm(ctx, h.repo, job)
}

func (h *SystemKeyRotateJobHandler) HandleJobDoneEvent(ctx context.Context, job orbital.Job) error {
	log.InjectSystemEvent(ctx, job.Type)

	data, err := unmarshalSystemJobData(job)
	if err != nil {
		return err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)
	system, err := getSystemByID(ctx, h.repo, data.SystemID)
	if err != nil {
		return err
	}

	// TODO: Add audit log when common-sdk provides CMK key rotation event
	// err = h.cmkAuditor.SendCmkKeyRotationAuditLog(ctx, system.Identifier, data.KeyIDTo,
	//     data.KeyVersionIDFrom, data.KeyVersionIDTo)
	// if err != nil {
	//     return fmt.Errorf("failed to send key rotation audit log for system %s: %w", system.ID, err)
	// }

	// Clean up event - rotation notification successfully sent to Kernel Service
	err = cleanUpEvent(ctx, h.repo, job)
	if err != nil {
		return fmt.Errorf("failed to clean up event for system %s: %w", system.ID, err)
	}

	return nil
}

func (h *SystemKeyRotateJobHandler) HandleJobFailedEvent(ctx context.Context, job orbital.Job) error {
	log.InjectSystemEvent(ctx, job.Type)

	data, err := unmarshalSystemJobData(job)
	if err != nil {
		return err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)
	system, err := getSystemByID(ctx, h.repo, data.SystemID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			log.Warn(ctx, "System not found when handling job termination, skipping event update",
				slog.String("systemID", data.SystemID))
			return nil
		}
		return err
	}

	// Merge task errors from Orbital to provide detailed failure context
	errorMessage, err := mergeOrbitalTaskErrors(ctx, h.orbitalManager, job)
	if err != nil {
		log.Error(ctx, "Failed to merge orbital task errors", err, slog.String("jobID", job.ID.String()))
		errorMessage = job.ErrorMessage // Fall back to the job error message
	}

	// Store error message in event for visibility
	err = updateEventError(ctx, h.repo, job.ExternalID, errorMessage)
	if err != nil {
		return err
	}

	// Note: We do NOT update system status to FAILED
	// Key rotation failure doesn't affect system connectivity - the system is still operational
	// It just means KS wasn't notified about the rotation yet

	log.Warn(ctx, "SYSTEM_KEY_ROTATE event failed - Kernel Service not notified of rotation",
		slog.String("systemID", system.Identifier),
		slog.String("keyID", data.KeyIDTo),
		slog.String("versionFrom", data.KeyVersionIDFrom),
		slog.String("versionTo", data.KeyVersionIDTo),
		slog.String("errorMessage", errorMessage))

	// TODO (future phase): Detect version mismatch errors from Kernel Service
	// If version mismatch detected:
	//   1. Trigger immediate version detection (don't wait for scheduled check)
	//   2. Create new SYSTEM_KEY_ROTATE event with updated version
	// This ensures eventual consistency even with rapid rotations

	return nil
}

func (h *SystemKeyRotateJobHandler) HandleJobCanceledEvent(ctx context.Context, job orbital.Job) error {
	log.InjectSystemEvent(ctx, job.Type)

	data, err := unmarshalSystemJobData(job)
	if err != nil {
		return err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)
	system, err := getSystemByID(ctx, h.repo, data.SystemID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			log.Warn(ctx, "System not found when handling job cancellation, skipping event update",
				slog.String("systemID", data.SystemID))
			return nil
		}
		return err
	}

	// Store cancellation reason in event
	err = updateEventError(ctx, h.repo, job.ExternalID, job.ErrorMessage)
	if err != nil {
		return err
	}

	log.Warn(ctx, "SYSTEM_KEY_ROTATE event canceled",
		slog.String("systemID", system.Identifier),
		slog.String("keyID", data.KeyIDTo),
		slog.String("errorMessage", job.ErrorMessage))

	return nil
}
