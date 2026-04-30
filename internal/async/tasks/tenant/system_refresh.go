package tasks

import (
	"context"

	"github.com/hibiken/asynq"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type SystemUpdater interface {
	UpdateSystems(ctx context.Context) error
}

type SystemsRefresher struct {
	systemClient SystemUpdater
	repo         repo.Repo
}

func NewSystemsRefresher(
	systemClient SystemUpdater,
	repo repo.Repo,
	opts ...async.TaskOption,
) async.TenantTaskHandler {
	s := &SystemsRefresher{
		systemClient: systemClient,
		repo:         repo,
	}
	for _, o := range opts {
		o(s)
	}

	return s
}

func (s *SystemsRefresher) ProcessTask(ctx context.Context, task *asynq.Task) error {
	ctx, err := cmkcontext.InjectInternalClientData(ctx,
		constants.InternalTaskSystemRefreshRole)
	if err != nil {
		s.logError(ctx, err)
		return nil
	}

	err = s.systemClient.UpdateSystems(ctx)

	// If network error return an error triggering
	// another task attempt with a backoff
	if isConnectionError(err) {
		return err
	}

	// Otherwise we log here and don't return an error for a retry
	if err != nil {
		s.logError(ctx, err)
	}
	return nil
}

func (s *SystemsRefresher) TenantQuery() *repo.Query {
	return repo.NewQuery()
}

func (s *SystemsRefresher) FanOutFunc() async.FanOutFunc {
	return async.TenantFanOut
}

func (s *SystemsRefresher) TaskType() string {
	return config.TypeSystemsTask
}

func (s *SystemsRefresher) logError(ctx context.Context, err error) {
	// Returned errors are retries in batch processor
	// If we don't want a retry we just log here and return nil
	log.Error(ctx, "Error during system refresh batch processing", err)
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
