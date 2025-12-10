package tasks

import (
	"context"

	"github.com/hibiken/asynq"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/log"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
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
) *SystemsRefresher {
	return &SystemsRefresher{
		systemClient: systemClient,
		repo:         repo,
	}
}

func (s *SystemsRefresher) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	var tenants []*model.Tenant

	_, err := s.repo.List(ctx, model.Tenant{}, &tenants, *repo.NewQuery())
	if err != nil {
		log.Error(ctx, "Getting Tenants on Refresh System Task", err)
		return nil
	}

	for _, tenant := range tenants {
		ctx := log.InjectTenant(cmkcontext.CreateTenantContext(ctx, tenant.ID), tenant)
		err = s.systemClient.UpdateSystems(ctx)
		// If network error return an error triggering
		// another task attempt with a backoff
		if isConnectionError(err) {
			return errs.Wrap(ErrRunningTask, err)
		}

		if err != nil {
			log.Error(ctx, "Running Refresh System Task", err)
		}
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
