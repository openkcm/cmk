package tasks

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"

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
	processor  *BatchProcessor
}

func NewHYOKSync(
	hyokClient HYOKUpdater,
	repo repo.Repo,
) *HYOKSync {
	return &HYOKSync{
		hyokClient: hyokClient,
		repo:       repo,
		processor:  NewBatchProcessor(repo),
	}
}

func (h *HYOKSync) ProcessTask(ctx context.Context, task *asynq.Task) error {
	log.Info(ctx, "Starting HYOK Sync Task")

	err := h.processor.ProcessTenantsInBatch(
		ctx,
		"HYOK Sync",
		task,
		repo.NewQuery(),
		func(tenantCtx context.Context, tenant *model.Tenant, index int) error {
			log.Debug(tenantCtx, "Syncing HYOK keys for tenant",
				slog.String("schemaName", tenant.SchemaName), slog.Int("index", index))

			syncErr := h.hyokClient.SyncHYOKKeys(ctx)
			if syncErr != nil {
				_ = h.handleErrorTask(ctx, syncErr)
				return nil
			}
			log.Debug(ctx, "HYOK keys for tenant synced successfully", slog.String("schemaName", tenant.SchemaName))
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
