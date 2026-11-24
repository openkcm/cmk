package tasks

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/repo"
)

type HyperscalerRenewer interface {
	RenewHyperscalerLease(ctx context.Context) error
}

type HyperscalerRenew struct {
	renewer HyperscalerRenewer
	repo    repo.Repo
}

func NewHyperscalerRenew(
	renewer HyperscalerRenewer,
	repo repo.Repo,
	opts ...async.TaskOption,
) async.TenantTaskHandler {
	h := &HyperscalerRenew{
		renewer: renewer,
		repo:    repo,
	}

	for _, o := range opts {
		o(h)
	}

	return h
}

func (h *HyperscalerRenew) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	err := h.renewer.RenewHyperscalerLease(ctx)
	if err != nil {
		// Log and return nil: a returned error is retried by the batch processor,
		// but one tenant's failure must not abort the whole run.
		log.Error(ctx, "Error during hyperscaler renewal batch processing", err)
	}
	return nil
}

func (h *HyperscalerRenew) TaskType() string {
	return config.TypeHyperscalerRenew
}

func (h *HyperscalerRenew) Role() constants.InternalRole {
	return constants.InternalTaskHyperscalerRenewRole
}

func (h *HyperscalerRenew) FanOutFunc() async.FanOutFunc {
	return async.TenantFanOut
}

func (h *HyperscalerRenew) TenantQuery() *repo.Query {
	return repo.NewQuery()
}
