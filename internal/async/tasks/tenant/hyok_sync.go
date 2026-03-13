package tasks

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/log"
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

func (h *HYOKSync) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return h.hyokClient.SyncHYOKKeys(ctx)
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

func (h *HYOKSync) TenantQuery() *repo.Query {
	return repo.NewQuery()
}

func (h *HYOKSync) handleErrorTenants(ctx context.Context, err error) error {
	log.Error(ctx, "Error during HYOK sync batch processing", err)
	return err
}
