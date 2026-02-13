package tasks

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	asyncUtils "github.com/openkcm/cmk/utils/async"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// BatchProcessor handles the common batch processing logic for async tasks
type BatchProcessor struct {
	repo repo.Repo
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(repo repo.Repo) *BatchProcessor {
	return &BatchProcessor{repo: repo}
}

// ProcessTenantsInBatch iterates through tenants in batches and applies the process function
// It tracks the total tenant count, logs batch progress, and logs task completion
func (bp *BatchProcessor) ProcessTenantsInBatch(
	ctx context.Context,
	taskName string,
	asynqTask *asynq.Task,
	query *repo.Query,
	processTenant func(ctx context.Context, tenant *model.Tenant, index int) error,
) error {
	totalTenantCount := 0

	var tenantIDs []string
	if asynqTask != nil && asynqTask.Payload() != nil {
		payload, err := asyncUtils.ParseTenantListPayload(asynqTask.Payload())
		if err != nil {
			log.Warn(ctx, "Failed to parse tenant IDs from task payload, processing all tenants")
		} else {
			log.Info(ctx, "Processing specified tenants for "+taskName, slog.Int("tenantCount", len(payload.TenantIDs)))
			tenantIDs = payload.TenantIDs
		}
	}

	if len(tenantIDs) > 0 {
		ck := repo.NewCompositeKey().Where(repo.IDField, tenantIDs)
		query = query.Where(repo.NewCompositeKeyGroup(ck))
	}

	err := repo.ProcessInBatch(ctx, bp.repo, query, repo.DefaultLimit,
		func(tenants []*model.Tenant) error {
			totalTenantCount += len(tenants)
			log.Debug(ctx, "Processing batch of tenants for "+taskName,
				slog.Int("batchSize", len(tenants)), slog.Int("totalTenantCount", totalTenantCount))

			for i, tenant := range tenants {
				tenantCtx := model.LogInjectTenant(cmkcontext.CreateTenantContext(ctx, tenant.ID), tenant)
				err := processTenant(tenantCtx, tenant, i+1)
				if err != nil {
					return err
				}
			}
			return nil
		})

	if err == nil {
		log.Info(ctx, taskName+" Task completed", slog.Int("totalTenantCount", totalTenantCount))
	}

	return err
}
