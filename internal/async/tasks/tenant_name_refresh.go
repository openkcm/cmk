package tasks

import (
	"context"

	"github.com/hibiken/asynq"

	tenantv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk/internal/clients/registry"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
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
	err := t.processor.ProcessTenantsInBatch(
		ctx,
		"Tenant Name Refresher",
		task,
		repo.NewQuery().Where(repo.NewCompositeKeyGroup(repo.NewCompositeKey().Where(repo.Name, repo.Empty))),
		func(ctx context.Context, tenant *model.Tenant, index int) error {
			res, err := t.registry.Tenant().GetTenant(ctx, &tenantv1.GetTenantRequest{
				Id: tenant.ID,
			})

			tenant.Name = res.GetTenant().GetName()
			// Log to not block other tenants if one fails
			if err != nil {
				log.Error(ctx, "Could not get tenant details", err)
			}

			_, err = t.r.Patch(ctx, tenant, *repo.NewQuery())
			if err != nil {
				return err
			}
			return nil
		},
	)
	if err != nil {
		log.Error(ctx, "Error during tenant name refresh batch processing", err)
		return errs.Wrap(ErrRunningTask, err)
	}
	return nil
}

func (t *TenantNameRefresher) TaskType() string {
	return config.TypeTenantRefreshName
}
