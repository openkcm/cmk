package tasks

import (
	"context"

	"github.com/hibiken/asynq"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/log"
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

	return s
}

func (s *SystemsRefresher) Process(ctx context.Context, _ *asynq.Task) error {
	return nil
}

// TODO: How to force task to run again
func (s *SystemsRefresher) ProcessTask(ctx context.Context, task *asynq.Task) error {
	err := s.systemClient.UpdateSystems(ctx)

	// If network error return an error triggering
	// another task attempt with a backoff
	if isConnectionError(err) {
		return err
	}

	if err != nil {
		log.Error(ctx, "Running Refresh System Task", err)
	}
	return nil
}

func (s *SystemsRefresher) TenantQuery() *repo.Query {
	return repo.NewQuery()
}

func (s *SystemsRefresher) SetFanOut(client async.Client, opts ...asynq.Option) {
	s.processor = async.NewBatchProcessor(s.repo, async.WithFanOutTenants(client, opts...))
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
