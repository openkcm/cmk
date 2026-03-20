package eventprocessor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/openkcm/orbital"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients/registry"
	"github.com/openkcm/cmk/internal/clients/registry/systems"
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

	err = sendL1KeyClaim(ctx, *system, h.registry, data.TenantID, true)
	if err != nil {
		return err
	}

	key, err := getKeyByKeyID(ctx, h.repo, data.KeyIDTo)
	if err != nil {
		return fmt.Errorf("failed to get key config ID for key %s: %w", data.KeyIDTo, err)
	}

	err = h.repo.Transaction(ctx, func(ctx context.Context) error {
		system.Status = cmkapi.SystemStatusCONNECTED
		system.KeyConfigurationID = &key.KeyConfigurationID
		err = updateSystem(ctx, h.repo, system)
		if err != nil {
			return err
		}

		return cleanUpEvent(ctx, h.repo, job)
	})
	if err != nil {
		return err
	}

	err = h.cmkAuditor.SendCmkOnboardingAuditLog(ctx, system.Identifier, data.KeyIDTo)
	if err != nil {
		log.Error(ctx, fmt.Sprintf("failed to send onboarding audit log for system %s: %v", system.ID, err), err)
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

	err = sendL1KeyClaim(ctx, *system, h.registry, data.TenantID, false)
	if err != nil {
		return err
	}

	err = h.repo.Transaction(ctx, func(ctx context.Context) error {
		system.Status = cmkapi.SystemStatusDISCONNECTED
		system.KeyConfigurationID = nil
		err := updateSystem(ctx, h.repo, system)
		if err != nil {
			return err
		}

		return cleanUpEvent(ctx, h.repo, job)
	})
	if err != nil {
		return err
	}

	err = h.cmkAuditor.SendCmkOffboardingAuditLog(ctx, system.Identifier, data.KeyIDFrom)
	if err != nil {
		log.Error(ctx, fmt.Sprintf("failed to send offboarding audit log for system %s: %v", system.ID, err), err)
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
	ctx = log.InjectSystemEvent(ctx, job.Type)

	data, err := unmarshalSystemJobData(job)
	if err != nil {
		return err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)
	system, err := getSystemByID(ctx, h.repo, data.SystemID)
	if err != nil {
		return err
	}

	key, err := getKeyByKeyID(ctx, h.repo, data.KeyIDTo)
	if err != nil {
		return fmt.Errorf("failed to get key config ID for key %s: %w", data.KeyIDTo, err)
	}

	err = h.repo.Transaction(ctx, func(ctx context.Context) error {
		system.KeyConfigurationID = &key.KeyConfigurationID
		err = updateSystem(ctx, h.repo, system)
		if err != nil {
			return err
		}

		return cleanUpEvent(ctx, h.repo, job)
	})
	if err != nil {
		return err
	}

	err = h.cmkAuditor.SendCmkSwitchAuditLog(ctx, system.Identifier, data.KeyIDFrom, data.KeyIDTo)
	if err != nil {
		log.Error(ctx, fmt.Sprintf("failed to send switch audit log for system %s: %v", system.ID, err), err)
	}

	return nil
}

func (h *SystemSwitchJobHandler) HandleJobFailedEvent(ctx context.Context, job orbital.Job) error {
	return terminateFailedSystemJob(ctx, h.orbitalManager, h.repo, job)
}

func (h *SystemSwitchJobHandler) HandleJobCanceledEvent(ctx context.Context, job orbital.Job) error {
	return terminateCanceledSystemJob(ctx, h.repo, job)
}

type SystemUnlinkDecommissionJobHandler struct {
	repo           repo.Repo
	registry       registry.Service
	cmkAuditor     *auditor.Auditor
	orbitalManager *orbital.Manager
	taskResolver   *SystemTaskInfoResolver
}

func NewSystemUnlinkDecommissionJobHandler(
	repo repo.Repo,
	registry registry.Service,
	cmkAuditor *auditor.Auditor,
	orbitalManager *orbital.Manager,
	taskResolver *SystemTaskInfoResolver,
) *SystemUnlinkDecommissionJobHandler {
	return &SystemUnlinkDecommissionJobHandler{
		repo:           repo,
		registry:       registry,
		cmkAuditor:     cmkAuditor,
		orbitalManager: orbitalManager,
		taskResolver:   taskResolver,
	}
}

func (h *SystemUnlinkDecommissionJobHandler) ResolveTasks(
	ctx context.Context,
	job orbital.Job,
) ([]orbital.TaskInfo, error) {
	return h.taskResolver.Resolve(ctx, job)
}

func (h *SystemUnlinkDecommissionJobHandler) HandleJobConfirm(
	ctx context.Context,
	job orbital.Job,
) (orbital.JobConfirmerResult, error) {
	return handleSystemJobConfirm(ctx, h.repo, job)
}

func (h *SystemUnlinkDecommissionJobHandler) HandleJobDoneEvent(ctx context.Context, job orbital.Job) error {
	system, keyID, err := h.terminate(ctx, job)
	if err != nil {
		return fmt.Errorf("failed to handle job done event for system unlink decommission job: %w", err)
	}

	err = h.cmkAuditor.SendCmkOffboardingAuditLog(ctx, system.Identifier, keyID)
	if err != nil {
		log.Error(ctx, fmt.Sprintf("failed to send offboarding audit log for system %s: %v", system.ID, err), err)
	}

	return nil
}

func (h *SystemUnlinkDecommissionJobHandler) HandleJobFailedEvent(ctx context.Context, job orbital.Job) error {
	taskErrorMessage, err := mergeOrbitalTaskErrors(ctx, h.orbitalManager, job)
	if err != nil {
		log.Error(ctx, "Failed to extract error message for failed system unlink decommission job", err)
		taskErrorMessage = "unknown error"
	}

	log.Warn(ctx, "System unlink decommission job failed, marking system as disconnected and cleaning up key claim",
		slog.String("errorMessage", taskErrorMessage),
	)

	_, _, err = h.terminate(ctx, job)
	if err != nil {
		return fmt.Errorf("failed to handle job failed event for system unlink decommission job: %w", err)
	}

	return nil
}

// HandleJobCanceledEvent puts the system to failed state so the system can be processed again during the next
// decommission reconciliation loop.
func (h *SystemUnlinkDecommissionJobHandler) HandleJobCanceledEvent(ctx context.Context, job orbital.Job) error {
	ctx = log.InjectSystemEvent(ctx, job.Type)

	data, err := unmarshalSystemJobData(job)
	if err != nil {
		return err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)
	system, err := getSystemByID(ctx, h.repo, data.SystemID)
	if err != nil {
		return err
	}

	system.Status = cmkapi.SystemStatusFAILED
	err = updateSystem(ctx, h.repo, system)
	if err != nil {
		return err
	}

	return nil
}

func (h *SystemUnlinkDecommissionJobHandler) terminate(
	ctx context.Context,
	job orbital.Job,
) (*model.System, string, error) {
	ctx = log.InjectSystemEvent(ctx, job.Type)

	data, err := unmarshalSystemJobData(job)
	if err != nil {
		return nil, "", err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)
	system, err := getSystemByID(ctx, h.repo, data.SystemID)
	if err != nil {
		return nil, "", err
	}

	// For unlink decommission, we want to mark the system as disconnected and clean up the key claim regardless
	// of whether the job succeeded or failed.
	system.Status = cmkapi.SystemStatusDISCONNECTED
	system.KeyConfigurationID = nil
	err = updateSystem(ctx, h.repo, system)
	if err != nil {
		return nil, "", err
	}

	err = sendL1KeyClaim(ctx, *system, h.registry, data.TenantID, false)
	if err != nil {
		return nil, "", err
	}

	return system, data.KeyIDFrom, nil
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
		log.Error(ctx, "Failed to merge orbital task errors", err)
		errorMessage = job.ErrorMessage // Fall back to the job error message if we fail to get task errors
	}

	err = r.Transaction(ctx, func(ctx context.Context) error {
		// Attempt to get task error messages from orbital to provide more context on the failure
		err := updateEventError(ctx, r, job.ExternalID, errorMessage)
		if err != nil {
			return err
		}

		system.Status = cmkapi.SystemStatusFAILED
		return updateSystem(ctx, r, system)
	})
	if err != nil {
		return err
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

	err = r.Transaction(ctx, func(ctx context.Context) error {
		// Attempt to get task error messages from orbital to provide more context on the cancellation
		err := updateEventError(ctx, r, job.ExternalID, job.ErrorMessage)
		if err != nil {
			return err
		}

		system.Status = cmkapi.SystemStatusFAILED
		return updateSystem(ctx, r, system)
	})
	if err != nil {
		return err
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
	ctx = log.InjectSystemEvent(ctx, job.Type)

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

	if err != nil {
		return fmt.Errorf("failed to clean up event for job %s: %w", job.ID, err)
	}

	return nil
}
