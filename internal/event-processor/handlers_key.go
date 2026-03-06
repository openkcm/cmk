package eventprocessor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/openkcm/orbital"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type KeyJobHandler struct {
	taskResolver *KeyTaskInfoResolver
}

func NewKeyJobHandler(taskResolver *KeyTaskInfoResolver) *KeyJobHandler {
	return &KeyJobHandler{
		taskResolver: taskResolver,
	}
}

func (h *KeyJobHandler) ResolveTasks(
	ctx context.Context,
	job orbital.Job,
) ([]orbital.TaskInfo, error) {
	return h.taskResolver.Resolve(ctx, job)
}

func (h *KeyJobHandler) HandleJobConfirm(
	_ context.Context,
	_ orbital.Job,
) (orbital.JobConfirmerResult, error) {
	return orbital.CompleteJobConfirmer(), nil
}

func (h *KeyJobHandler) HandleJobDoneEvent(
	_ context.Context,
	_ orbital.Job,
) error {
	return nil
}

func (h *KeyJobHandler) HandleJobFailedEvent(
	_ context.Context,
	_ orbital.Job,
) error {
	return nil
}

func (h *KeyJobHandler) HandleJobCanceledEvent(
	_ context.Context,
	_ orbital.Job,
) error {
	return nil
}

type KeyDetachJobHandler struct {
	repo           repo.Repo
	cmkAuditor     *auditor.Auditor
	orbitalManager *orbital.Manager
	taskResolver   *KeyTaskInfoResolver
}

func NewKeyDetachJobHandler(
	repo repo.Repo,
	cmkAuditor *auditor.Auditor,
	orbitalManager *orbital.Manager,
	taskResolver *KeyTaskInfoResolver,
) *KeyDetachJobHandler {
	return &KeyDetachJobHandler{
		repo:           repo,
		cmkAuditor:     cmkAuditor,
		orbitalManager: orbitalManager,
		taskResolver:   taskResolver,
	}
}

func (h *KeyDetachJobHandler) ResolveTasks(
	ctx context.Context,
	job orbital.Job,
) ([]orbital.TaskInfo, error) {
	return h.taskResolver.Resolve(ctx, job)
}

func (h *KeyDetachJobHandler) HandleJobConfirm(
	_ context.Context,
	_ orbital.Job,
) (orbital.JobConfirmerResult, error) {
	return orbital.CompleteJobConfirmer(), nil
}

func (h *KeyDetachJobHandler) HandleJobDoneEvent(
	ctx context.Context,
	job orbital.Job,
) error {
	key, err := h.terminate(ctx, job, cmkapi.KeyStateDETACHED)
	if err != nil {
		return fmt.Errorf("failed to update key state on detach job completion: %w", err)
	}

	err = h.cmkAuditor.SendCmkDetachAuditLog(ctx, key.ID.String())
	if err != nil {
		log.Error(ctx, "Failed to send audit log for key detach", err)
	}

	return nil
}

func (h *KeyDetachJobHandler) HandleJobFailedEvent(
	ctx context.Context,
	job orbital.Job,
) error {
	taskErrorMessage, err := mergeOrbitalTaskErrors(ctx, h.orbitalManager, job)
	if err != nil {
		log.Error(ctx, "Failed to extract error message for failed key detach job", err)
		taskErrorMessage = "unknown error"
	}

	log.Warn(ctx, "Key detach job failed, ignoring and updating key state to DETACHED",
		slog.String("errorMessage", taskErrorMessage),
	)

	_, err = h.terminate(ctx, job, cmkapi.KeyStateDETACHED)
	if err != nil {
		return fmt.Errorf("failed to update key state on detach job failure: %w", err)
	}

	return nil
}

func (h *KeyDetachJobHandler) HandleJobCanceledEvent(
	ctx context.Context,
	job orbital.Job,
) error {
	log.Info(ctx, "Key detach job was canceled, updating key state to UNKNOWN to be retried later")

	_, err := h.terminate(ctx, job, cmkapi.KeyStateUNKNOWN)
	if err != nil {
		return fmt.Errorf("failed to update key state on detach job cancellation: %w", err)
	}

	return nil
}

func (h *KeyDetachJobHandler) terminate(
	ctx context.Context,
	job orbital.Job,
	targetState cmkapi.KeyState,
) (*model.Key, error) {
	data, err := unmarshalKeyJobData(job)
	if err != nil {
		return nil, err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)
	key, err := getKeyByKeyID(ctx, h.repo, data.KeyID)
	if err != nil {
		return nil, err
	}

	key.State = string(targetState)
	err = updateKey(ctx, h.repo, key)
	if err != nil {
		return nil, err
	}

	return key, nil
}
