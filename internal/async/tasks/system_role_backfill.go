package tasks

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/repo"
	asyncUtils "github.com/openkcm/cmk/utils/async"
)

// SystemRoleUpdater is implemented by SystemInformation.
type SystemRoleUpdater interface {
	UpdateSystemByExternalID(ctx context.Context, externalID string) error
	HasEmptyRoleByExternalID(ctx context.Context, externalID string) (bool, error)
}

// SystemRoleBackfill is an on-demand per-system task that re-runs enrichment for
// a single subaccount whose role data was not yet available at registration.
// It returns an error to retry (with backoff) while the role is still empty, so
// the system self-heals within minutes rather than at the next hourly refresh.
type SystemRoleBackfill struct {
	systemClient SystemRoleUpdater
	repo         repo.Repo
}

func NewSystemRoleBackfill(
	systemClient SystemRoleUpdater,
	repo repo.Repo,
) async.TaskHandler {
	return &SystemRoleBackfill{
		systemClient: systemClient,
		repo:         repo,
	}
}

func (h *SystemRoleBackfill) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := asyncUtils.ParseTaskPayload(task.Payload())
	if err != nil {
		log.Error(ctx, "Failed to parse system role backfill task payload", err)
		return err
	}

	externalID := string(payload.Data)

	ctx = payload.InjectContext(ctx)

	log.Info(ctx, "Starting system role backfill task", slog.String("externalID", externalID))

	err = h.systemClient.UpdateSystemByExternalID(ctx, externalID)
	if err != nil {
		log.Info(ctx, "System role backfill will retry", slog.String("externalID", externalID), log.ErrorAttr(err))
		return err
	}

	empty, err := h.systemClient.HasEmptyRoleByExternalID(ctx, externalID)
	if err != nil {
		log.Info(ctx, "System role backfill will retry", slog.String("externalID", externalID), log.ErrorAttr(err))
		return err
	}

	if empty {
		log.Info(ctx, "System role still empty, backfill will retry", slog.String("externalID", externalID))
		return ErrRoleStillEmpty
	}

	log.Info(ctx, "System role backfill completed", slog.String("externalID", externalID))
	return nil
}

func (h *SystemRoleBackfill) TaskType() string {
	return config.TypeSystemRoleBackfill
}

func (h *SystemRoleBackfill) Role() constants.InternalRole {
	return constants.InternalTaskSystemRoleBackfillRole
}
