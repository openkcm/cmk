package tasks

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type HYOKUpdater interface {
	SyncHYOKKeys(ctx context.Context) error
}

type HYOKSync struct {
	hyokClient HYOKUpdater
	repo       repo.Repo
}

func NewHYOKSync(
	hyokClient HYOKUpdater,
	repo repo.Repo,
	opts ...async.TaskOption,
) async.TenantTaskHandler {
	h := &HYOKSync{
		hyokClient: hyokClient,
		repo:       repo,
	}

	for _, o := range opts {
		o(h)
	}

	return h
}

func (h *HYOKSync) ProcessTask(ctx context.Context, task *asynq.Task) error {
	ctx, err := cmkcontext.InjectInternalClientData(ctx,
		constants.InternalTaskHYOKSyncRole)
	if err != nil {
		h.logError(ctx, err)
		return nil
	}

	err = h.hyokClient.SyncHYOKKeys(ctx)
	if err != nil {
		h.logError(ctx, err)
	}
	return nil
}

func (h *HYOKSync) TaskType() string {
	return config.TypeHYOKSync
}

func (h *HYOKSync) FanOutFunc() async.FanOutFunc {
	return async.TenantFanOut
}

func (h *HYOKSync) TenantQuery() *repo.Query {
	return repo.NewQuery()
}

func (h *HYOKSync) logError(ctx context.Context, err error) {
	// Returned errors are retries in batch processor
	// If we don't want a retry we just log here and return nil
	log.Error(ctx, "Error during hyok sync batch processing", err)
}
