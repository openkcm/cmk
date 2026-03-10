package async

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	asyncUtils "github.com/openkcm/cmk/utils/async"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type Options func(*BatchProcessor)

func WithFanOut(asyncClient Client) Options {
	return func(bp *BatchProcessor) {
		bp.fanOutMode = true
		bp.asyncClient = asyncClient
	}
}

type BatchProcessor struct {
	repo        repo.Repo
	asyncClient Client
	fanOutMode  bool
}

func NewBatchProcessor(repo repo.Repo, opts ...Options) *BatchProcessor {
	bp := &BatchProcessor{
		repo:       repo,
		fanOutMode: false,
	}

	for _, o := range opts {
		o(bp)
	}

	return bp
}

// ProcessTenantsInBatch iterates through tenants in batches and applies the process function
// It tracks the total tenant count, logs batch progress, and logs task completion
// In fan-out mode, it enqueues child tasks instead of processing inline
func (bp *BatchProcessor) ProcessTenantsInBatch(
	ctx context.Context,
	asynqTask *asynq.Task,
	query *repo.Query,
	processTenant func(ctx context.Context, tenant *model.Tenant, index int) error,
) error {
	totalTenantCount := 0

	ctx = slogctx.With(ctx,
		slog.String("task", asynqTask.Type()),
	)

	var tenantIDs []string
	if asynqTask != nil && asynqTask.Payload() != nil {
		payload, err := asyncUtils.ParseTenantListPayload(asynqTask.Payload())
		if err != nil {
			log.Warn(ctx, "Failed to parse tenant IDs from task payload, processing all tenants")
		} else {
			log.Info(
				ctx,
				"Processing specified tenants",
				slog.Int("tenantCount", len(payload.TenantIDs)),
			)
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

			ctx := slogctx.With(ctx,
				slog.Int("batchSize", len(tenants)),
				slog.Int("totalTenantCount", totalTenantCount),
			)
			log.Debug(ctx, "Processing batch of tenants ")

			for i, tenant := range tenants {
				ctx := cmkcontext.New(
					ctx,
					cmkcontext.WithTenant(tenant.ID),
					cmkcontext.InjectSystemUser,
					model.WithLogInjectTenant(tenant),
					log.WithLogInjectAttrs(slog.Int("index", i)),
				)

				if !bp.fanOutMode {
					log.Debug(ctx, "Starting async task processing")
					err := processTenant(ctx, tenant, i+1)
					if err != nil {
						return err
					}
					log.Debug(ctx, "Finished async task processig")
				} else {
					log.Debug(ctx, "Creating Fanned-Out Task")
					err := FanOutTask(
						ctx,
						bp.asyncClient,
						asynqTask,
						asyncUtils.NewTaskPayload(ctx, asynqTask.Payload()),
					)
					if err != nil {
						return err
					}
					log.Debug(ctx, "Created Fanned-Out Task")
				}
			}
			return nil
		})

	if err == nil {
		log.Info(ctx, "Task completed", slog.Int("totalTenantCount", totalTenantCount))
	}

	return err
}
