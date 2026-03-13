package tasks

import (
	"context"

	"github.com/hibiken/asynq"

	tenantv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/clients/registry"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type TenantNameRefresher struct {
	processor *async.BatchProcessor
	r         repo.Repo
	registry  registry.Service
}

func NewTenantNameRefresher(
	r repo.Repo,
	registry registry.Service,
	opts ...async.TaskOption,
) async.TenantTaskHandler {
	t := &TenantNameRefresher{
		processor: async.NewBatchProcessor(r),
		registry:  registry,
		r:         r,
	}
	for _, o := range opts {
		o(t)
	}

	return t
}

func (t *TenantNameRefresher) ProcessTask(ctx context.Context, task *asynq.Task) error {
	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return err
	}
	res, err := t.registry.Tenant().GetTenant(ctx, &tenantv1.GetTenantRequest{
		Id: tenantID,
	})
	if err != nil {
		return errs.Wrapf(err, "Could not get tenant details")
	}

	tenant := &model.Tenant{
		ID:   tenantID,
		Name: res.GetTenant().GetName(),
	}
	_, err = t.r.Patch(ctx, tenant, *repo.NewQuery())
	if err != nil {
		return err
	}
	return nil
}

func (t *TenantNameRefresher) TenantQuery() *repo.Query {
	return repo.NewQuery().Where(repo.NewCompositeKeyGroup(repo.NewCompositeKey().Where(repo.Name, repo.Empty)))
}

func (t *TenantNameRefresher) FanOutFunc() async.FunOutFunc {
	return async.TenantFanOut
}

func (t *TenantNameRefresher) TaskType() string {
	return config.TypeTenantRefreshName
}
