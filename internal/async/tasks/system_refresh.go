package tasks

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hibiken/asynq"
	"github.com/openkcm/cmk/internal/async"
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
	processor    *async.BatchProcessor
	fanout       bool
}

func NewSystemsRefresher(
	systemClient SystemUpdater,
	repo repo.Repo,
	opts ...async.TaskOption,
) *SystemsRefresher {
	s := &SystemsRefresher{
		systemClient: systemClient,
		repo:         repo,
		processor:    async.NewBatchProcessor(repo),
	}
	for _, o := range opts {
		o(s)
	}

	log.Debug(context.Background(), "Created System Refresh Task")

	return s
}

func (s *SystemsRefresher) ProcessTask(ctx context.Context, task *asynq.Task) error {
	log.Info(ctx, "Starting Systems Refresh Task")

	if async.IsChildTask(task) {
		return async.ProcessChildTask(ctx, task, s.process)
	}

	err := s.processor.ProcessTenantsInBatch(
		ctx,
		task,
		repo.NewQuery(),
		func(ctx context.Context, _ *model.Tenant) error {
			return s.process(ctx)
		},
	)
	if err != nil {
		log.Error(ctx, "Error during systems refresh batch processing", err)
		return errs.Wrap(ErrRunningTask, err)
	}

	return nil
}

func (s *SystemsRefresher) process(ctx context.Context) error {
	updateErr := s.systemClient.UpdateSystems(ctx)
	// If network error return an error triggering
	// another task attempt with a backoff
	if isConnectionError(updateErr) {
		return updateErr
	}

	if updateErr != nil {
		log.Error(ctx, "Running Refresh System Task", updateErr)
	}
	return nil
}

func (s *SystemsRefresher) SetFanOut(client async.Client) {
	s.processor = async.NewBatchProcessor(s.repo, async.WithFanOutTenants(client))
	s.fanout = true
}

func (s *SystemsRefresher) IsFanOutEnabled() bool {
	return s.fanout
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
