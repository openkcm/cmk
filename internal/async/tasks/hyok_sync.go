package tasks

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
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

	err := h.processor.ProcessTenantsInBatchWithOptions(
		ctx,
		"HYOK Sync",
		task,
		repo.NewQuery(),
		repo.BatchProcessOptions{IgnoreFailMode: true},
		func(ctx context.Context, tenant *model.Tenant, index int) error {
			log.Debug(ctx, "Syncing HYOK keys for tenant",
				slog.String("schemaName", tenant.SchemaName), slog.Int("index", index))

			ctx, err := cmkcontext.InjectInternalClientData(ctx,
				constants.InternalTaskHYOKSyncRole)
			if err != nil {
				return h.handleErrorTask(ctx, err)
			}

			syncErr := h.hyokClient.SyncHYOKKeys(ctx)
			if syncErr != nil {
				return h.handleErrorTask(ctx, syncErr)
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
