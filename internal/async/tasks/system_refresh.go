package tasks

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
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

	err := s.processor.ProcessTenantsInBatchWithOptions(
		ctx,
		"Systems Refresh",
		task,
		repo.NewQuery(),
		repo.BatchProcessOptions{IgnoreFailMode: true},
		func(tenantCtx context.Context, tenant *model.Tenant, index int) error {
			log.Debug(tenantCtx, "Refreshing systems for tenant",
				slog.String("schemaName", tenant.SchemaName), slog.Int("index", index))

			ctx, err := cmkcontext.InjectInternalClientData(ctx,
				constants.InternalTaskSystemRefreshRole)
			if err != nil {
				return s.handleErrorTask(ctx, err)
			}

			updateErr := s.systemClient.UpdateSystems(ctx)
			// If network error return an error triggering
			// another task attempt with a backoff
			if isConnectionError(updateErr) {
				return updateErr
			}

			if updateErr != nil {
				return s.handleErrorTask(ctx, updateErr)
			}
			return nil
		})
	if err != nil {
		return s.handleErrorTenants(ctx, err)
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

func (s *SystemsRefresher) handleErrorTenants(ctx context.Context, err error) error {
	log.Error(ctx, "Error during system refresh sync batch processing", err)
	return errs.Wrap(ErrRunningTask, err)
}

func (s *SystemsRefresher) handleErrorTask(ctx context.Context, err error) error {
	log.Error(ctx, "Running system refresh", err)
	return errs.Wrap(ErrRunningTask, err)
}
