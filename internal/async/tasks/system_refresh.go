package tasks

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

type SystemUpdater interface {
	UpdateSystems(ctx context.Context) error
}

type SystemsRefresher struct {
	systemClient SystemUpdater
	repo         repo.Repo
	processor    *BatchProcessor
}

func NewSystemsRefresher(
	systemClient SystemUpdater,
	repo repo.Repo,
) *SystemsRefresher {
	return &SystemsRefresher{
		systemClient: systemClient,
		repo:         repo,
		processor:    NewBatchProcessor(repo),
	}
}

func (s *SystemsRefresher) ProcessTask(ctx context.Context, task *asynq.Task) error {
	log.Info(ctx, "Starting Systems Refresh Task")

	err := s.processor.ProcessTenantsInBatch(ctx, "Systems Refresh", task,
		func(tenantCtx context.Context, tenant *model.Tenant, index int) error {
			log.Debug(tenantCtx, "Refreshing systems for tenant",
				slog.String("schemaName", tenant.SchemaName), slog.Int("index", index))

			updateErr := s.systemClient.UpdateSystems(tenantCtx)
			// If network error return an error triggering
			// another task attempt with a backoff
			if isConnectionError(updateErr) {
				return updateErr
			}

			if updateErr != nil {
				log.Error(tenantCtx, "Running Refresh System Task", updateErr)
			}
			return nil
		})

	if err != nil {
		log.Error(ctx, "Error during systems refresh batch processing", err)
		return errs.Wrap(ErrRunningTask, err)
	}

	return nil
}

func (s *SystemsRefresher) TaskType() string {
	return config.TypeSystemsTask
}

// Checks if gRPC error is of the network type
// https://grpc.io/docs/guides/error/
func isConnectionError(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}

	code := st.Code()

	return code == codes.Unavailable || code == codes.DeadlineExceeded
}
