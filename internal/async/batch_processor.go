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

type BatchProcessorOptions func(*BatchProcessor)

func WithFanOutTenants(asyncClient Client, opts ...asynq.Option) BatchProcessorOptions {
	return func(bp *BatchProcessor) {
		bp.fanOut = true
		bp.asyncClient = asyncClient
		bp.fanOutOpts = opts
	}
}

func WithTenantQuery(q *repo.Query) BatchProcessorOptions {
	return func(bp *BatchProcessor) {
		bp.tenantQuery = q
	}
}

type BatchProcessor struct {
	repo        repo.Repo
	asyncClient Client
	tenantQuery *repo.Query
	fanOut      bool
	fanOutOpts  []asynq.Option
}

func NewBatchProcessor(r repo.Repo, opts ...BatchProcessorOptions) *BatchProcessor {
	bp := &BatchProcessor{
		repo:        r,
		fanOut:      false,
		tenantQuery: repo.NewQuery(),
	}

	for _, o := range opts {
		o(bp)
	}

	return bp
}

// ProcessTenantsInBatch iterates through tenants in batches and applies the process function
// It tracks the total tenant count, logs batch progress, and logs task completion
// In fan-out mode, it enqueues child tasks instead of processing inline
//
//nolint:funlen, cyclop
func (bp *BatchProcessor) ProcessTenantsInBatch(
	ctx context.Context,
	asynqTask *asynq.Task,
	processTenant func(ctx context.Context, task *asynq.Task) error,
) error {
	totalTenantCount := 0

	if asynqTask == nil {
		return ErrNilTask
	}

	ctx = slogctx.With(ctx,
		slog.String("task", asynqTask.Type()),
	)

	var tenantIDs []string
	if asynqTask.Payload() != nil {
		payload, err := asyncUtils.ParseTenantListPayload(asynqTask.Payload())
		if err != nil {
			log.Error(ctx, "failed to parse the tenant list", err)
			return err
		}

		if len(payload.TenantIDs) > 0 {
			log.Info(
				ctx,
				"Processing specified tenants",
				slog.Int("tenantCount", len(payload.TenantIDs)),
			)
			tenantIDs = payload.TenantIDs
		} else {
			log.Warn(ctx, "Failed to parse tenant IDs from task payload, processing all tenants")
		}
	}

	query := bp.tenantQuery
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

			for i, tenant := range tenants {
				ctx := cmkcontext.New(
					ctx,
					cmkcontext.WithTenant(tenant.ID),
					cmkcontext.InjectSystemUser,
					model.WithLogInjectTenant(tenant),
					log.WithLogInjectAttrs(slog.Int("index", i)),
				)

				if !bp.fanOut {
					err := processTenant(ctx, asynqTask)
					if err != nil {
						log.Error(ctx, "Task failed", err)
					}
				} else {
					// Create child task with tenant information in payload
					payload := asyncUtils.NewTaskPayload(ctx, asynqTask.Payload())
					err := FanOutTask(
						bp.asyncClient,
						asynqTask,
						payload,
						bp.fanOutOpts...,
					)
					if err != nil {
						return err
					}
				}
			}
			return nil
		})

	if err == nil {
		log.Info(ctx, "Task completed", slog.Int("totalTenantCount", totalTenantCount))
	}

	return err
}
