package tasks

import (
	"context"

	"github.com/hibiken/asynq"

	tenantv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/clients/registry"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type TenantNameRefresher struct {
	processor *async.BatchProcessor
	r         repo.Repo
	registry  registry.Service
	fanout    bool
}

func NewTenantNameRefresher(
	r repo.Repo,
	registry registry.Service,
	opts ...async.TaskOption,
) *TenantNameRefresher {
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
	log.Info(ctx, "Starting Tenant Name Refresh Task")

	if async.IsChildTask(task) {
		return t.processChildTask(ctx, task)
	}

	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(repo.NewCompositeKey().Where(repo.Name, repo.Empty)))

	err := t.processor.ProcessTenantsInBatch(
		ctx,
		task,
		query,
		t.process,
	)
	if err != nil {
		log.Error(ctx, "Error during tenant name refresh batch processing", err)
		return errs.Wrap(ErrRunningTask, err)
	}
	return nil
}

func (t *TenantNameRefresher) SetFanOut(client async.Client, opts ...asynq.Option) {
	t.processor = async.NewBatchProcessor(t.r, async.WithFanOutTenants(client, opts...))
	t.fanout = true
}

func (t *TenantNameRefresher) IsFanOutEnabled() bool {
	return t.fanout
}

func (t *TenantNameRefresher) TaskType() string {
	return config.TypeTenantRefreshName
}

func (t *TenantNameRefresher) process(ctx context.Context) error {
	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return err
	}
	res, err := t.registry.Tenant().GetTenant(ctx, &tenantv1.GetTenantRequest{
		Id: tenantID,
	})
	// Log to not block other tenants if one fails
	if err != nil {
		log.Error(ctx, "Could not get tenant details", err)
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

func (t *TenantNameRefresher) processChildTask(ctx context.Context, task *asynq.Task) error {
	return async.ProcessChildTask(ctx, task, func(ctx context.Context) error {
		return t.process(ctx)
	})
}
