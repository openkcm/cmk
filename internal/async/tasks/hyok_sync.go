package tasks

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

type HYOKUpdater interface {
	SyncHYOKKeys(ctx context.Context) error
}

type HYOKSync struct {
	hyokClient HYOKUpdater
	repo       repo.Repo
	processor  *async.BatchProcessor
}

func NewHYOKSync(
	hyokClient HYOKUpdater,
	repo repo.Repo,
	asyncClient async.Client,
) *HYOKSync {
	var bp *async.BatchProcessor
	if asyncClient == nil {
		bp = async.NewBatchProcessor(repo)
	} else {
		bp = async.NewBatchProcessor(repo, async.WithFanOut(asyncClient))
	}
	log.Debug(context.Background(), "Created HYOK Sync Client Task", slog.Bool("fanOut", asyncClient != nil))

	return &HYOKSync{
		hyokClient: hyokClient,
		repo:       repo,
		processor:  bp,
	}
}

func (h *HYOKSync) ProcessTask(ctx context.Context, task *asynq.Task) error {
	err := h.processor.ProcessTenantsInBatch(
		ctx,
		task,
		repo.NewQuery(),
		func(ctx context.Context, tenant *model.Tenant, index int) error {
			syncErr := h.hyokClient.SyncHYOKKeys(ctx)
			if syncErr != nil {
				_ = h.handleErrorTask(ctx, syncErr)
				return nil
			}
			return nil
		},
	)
	if err != nil {
		return h.handleErrorTenants(ctx, err)
	}

	return nil
}

func (h *HYOKSync) TaskType() string {
	return config.TypeHYOKSync
}

func (h *HYOKSync) handleErrorTenants(ctx context.Context, err error) error {
	log.Error(ctx, "Error during HYOK sync batch processing", err)
	return errs.Wrap(ErrRunningTask, err)
}

func (h *HYOKSync) handleErrorTask(ctx context.Context, err error) error {
	log.Error(ctx, "Running HYOK Sync", err)
	return errs.Wrap(ErrRunningTask, err)
}
