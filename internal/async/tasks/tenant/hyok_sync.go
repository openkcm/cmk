package tasks

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/repo"
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

func (h *HYOKSync) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return h.hyokClient.SyncHYOKKeys(ctx)
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
