package tasks

import (
	"context"

	"github.com/hibiken/asynq"

	tenantv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk/internal/clients/registry"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type TenantNameRefresher struct {
	processor *BatchProcessor
	r         repo.Repo
	registry  registry.Service
}

func NewTenantNameRefresher(r repo.Repo, registry registry.Service) *TenantNameRefresher {
	return &TenantNameRefresher{
		processor: NewBatchProcessor(r),
		registry:  registry,
		r:         r,
	}
}

func (t *TenantNameRefresher) ProcessTask(ctx context.Context, task *asynq.Task) error {
	err := t.processor.ProcessTenantsInBatchWithOptions(
		ctx,
		"Tenant Name Refresher",
		task,
		repo.NewQuery().Where(repo.NewCompositeKeyGroup(repo.NewCompositeKey().Where(repo.Name, repo.Empty))),
		repo.BatchProcessOptions{IgnoreFailMode: true},
		func(ctx context.Context, tenant *model.Tenant, index int) error {
			ctx, err := cmkcontext.InjectInternalClientData(ctx,
				constants.InternalTaskTenantRefreshRole)
			if err != nil {
				return t.handleErrorTask(ctx, err)
			}

			res, err := t.registry.Tenant().GetTenant(ctx, &tenantv1.GetTenantRequest{
				Id: tenant.ID,
			})
			if err != nil {
				return t.handleErrorTask(ctx, err)
			}

			tenant.Name = res.GetTenant().GetName()

			_, err = t.r.Patch(ctx, tenant, *repo.NewQuery())
			if err != nil {
				return t.handleErrorTask(ctx, err)
			}
			return nil
		},
	)
	if err != nil {
		return t.handleErrorTenants(ctx, err)
	}
	return nil
}

func (t *TenantNameRefresher) TaskType() string {
	return config.TypeTenantRefreshName
}

func (t *TenantNameRefresher) handleErrorTenants(ctx context.Context, err error) error {
	log.Error(ctx, "Error during tenant refresh sync batch processing", err)
	return errs.Wrap(ErrRunningTask, err)
}

func (t *TenantNameRefresher) handleErrorTask(ctx context.Context, err error) error {
	log.Error(ctx, "Running tenant refresh", err)
	return errs.Wrap(ErrRunningTask, err)
}
