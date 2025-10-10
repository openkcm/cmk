package tasks

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/config"
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
}

func NewHYOKSync(
	hyokClient HYOKUpdater,
	repo repo.Repo,
) *HYOKSync {
	return &HYOKSync{
		hyokClient: hyokClient,
		repo:       repo,
	}
}

func (h *HYOKSync) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	var tenants []*model.Tenant

	_, err := h.repo.List(ctx, model.Tenant{}, &tenants, *repo.NewQuery())
	if err != nil {
		return h.handleErrorTenants(ctx, err)
	}

	for _, tenant := range tenants {
		ctx := log.InjectTenant(cmkcontext.CreateTenantContext(ctx, tenant.ID), tenant)
		log.Debug(ctx, "Syncing HYOK keys")

		err = h.hyokClient.SyncHYOKKeys(ctx)
		if err != nil {
			_ = h.handleErrorTask(ctx, err)
			continue
		}
	}

	return nil
}

func (h *HYOKSync) TaskType() string {
	return config.TypeHYOKSync
}

func (h *HYOKSync) handleErrorTenants(ctx context.Context, err error) error {
	log.Error(ctx, "Getting tenants for HYOK Sync", err)
	return errs.Wrap(ErrRunningTask, err)
}

func (h *HYOKSync) handleErrorTask(ctx context.Context, err error) error {
	log.Error(ctx, "Running HYOK Sync", err)
	return errs.Wrap(ErrRunningTask, err)
}
