package tasks

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/repo"
	asyncUtils "github.com/openkcm/cmk/utils/async"
)

// PendingStateUpdater is implemented by KeyManager.
type PendingStateUpdater interface {
	SyncPendingCreationKey(ctx context.Context, keyID uuid.UUID) error
}

// PendingStateSync is an on-demand per-key task that processes a single key in
// PENDING_CREATION state. It completes provisioning once provider-side prerequisites
// are met, or transitions the key to ERROR on hard timeout.
type PendingStateSync struct {
	keyClient PendingStateUpdater
	repo      repo.Repo
}

func NewPendingStateSync(
	keyClient PendingStateUpdater,
	repo repo.Repo,
) async.TaskHandler {
	return &PendingStateSync{
		keyClient: keyClient,
		repo:      repo,
	}
}

func (h *PendingStateSync) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := asyncUtils.ParseTaskPayload(task.Payload())
	if err != nil {
		log.Error(ctx, "Failed to parse pending state sync task payload", err)
		return err
	}

	keyID, err := uuid.ParseBytes(payload.Data)
	if err != nil {
		log.Error(ctx, "Failed to parse key ID from pending state sync payload", err)
		return err
	}

	ctx = payload.InjectContext(ctx)

	log.Info(ctx, "Starting pending state sync task", slog.String("keyID", keyID.String()))

	if err := h.keyClient.SyncPendingCreationKey(ctx, keyID); err != nil {
		log.Info(ctx, "Pending state sync will retry", slog.String("keyID", keyID.String()), log.ErrorAttr(err))
		return err
	}

	log.Info(ctx, "Pending state sync completed", slog.String("keyID", keyID.String()))
	return nil
}

func (h *PendingStateSync) TaskType() string {
	return config.TypePendingStateSync
}

func (h *PendingStateSync) Role() constants.InternalRole {
	return constants.InternalTaskPendingStateSyncRole
}
