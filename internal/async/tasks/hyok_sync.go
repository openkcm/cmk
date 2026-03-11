package tasks

import (
	"context"

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
	fanout     bool
}

func NewHYOKSync(
	hyokClient HYOKUpdater,
	repo repo.Repo,
	opts ...async.TaskOption,
) async.TaskHandler {
	h := &HYOKSync{
		hyokClient: hyokClient,
		repo:       repo,
		processor:  async.NewBatchProcessor(repo),
	}

	for _, o := range opts {
		o(h)
	}

	return h
}

func (h *HYOKSync) ProcessTask(ctx context.Context, task *asynq.Task) error {
	log.Info(ctx, "Starting HYOK Sync Task")

	if async.IsChildTask(task) {
		return async.ProcessChildTask(ctx, task, h.process)
	}

	err := h.processor.ProcessTenantsInBatch(
		ctx,
		task,
		repo.NewQuery(),
		func(ctx context.Context, _ *model.Tenant) error {
			return h.process(ctx)
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

func (h *HYOKSync) SetFanOut(client async.Client, opts ...asynq.Option) {
	h.processor = async.NewBatchProcessor(h.repo, async.WithFanOutTenants(client, opts...))
	h.fanout = true
}

func (h *HYOKSync) IsFanOutEnabled() bool {
	return h.fanout
}

func (h *HYOKSync) handleErrorTenants(ctx context.Context, err error) error {
	log.Error(ctx, "Error during HYOK sync batch processing", err)
	return errs.Wrap(ErrRunningTask, err)
}

func (h *HYOKSync) process(ctx context.Context) error {
	syncErr := h.hyokClient.SyncHYOKKeys(ctx)
	if syncErr != nil {
		log.Error(ctx, "Running HYOK Sync", syncErr)
	}
	return nil
}
