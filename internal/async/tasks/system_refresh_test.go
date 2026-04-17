package tasks_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openkcm/cmk/internal/async/tasks"
	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

var allowedSystemRefreshActions = []authz.RepoAction{
	authz.RepoActionCount,
	authz.RepoActionList,
	authz.RepoActionUpdate,
}

type SystemUpdaterMock struct {
	CallCount   int
	authzLoader *authz_loader.AuthzLoader[authz.RepoResourceTypeName,
		authz.RepoAction]
}

//nolint:wrapcheck
func (s *SystemUpdaterMock) UpdateSystems(ctx context.Context) error {
	s.CallCount += 1
	// We only test a subset of the permissions
	for _, testAction := range allowedSystemRefreshActions {
		isAllowed, err := authz.CheckAuthz(ctx, s.authzLoader.AuthzHandler,
			authz.RepoResourceTypeSystemProperty, testAction)
		if err != nil {
			return err
		}
		if !isAllowed {
			return authz.ErrAuthzDecision
		}
	}
	return nil
}

type SystemUpdaterMockError struct {
	CallCount int
}

//nolint:wrapcheck
func (s *SystemUpdaterMockError) UpdateSystems(ctx context.Context) error {
	s.CallCount += 1
	st := status.New(codes.DeadlineExceeded, "timeout")
	return st.Err()
}

type SystemUpdaterMockUnauthz struct {
	CallCount   int
	authzLoader *authz_loader.AuthzLoader[authz.RepoResourceTypeName,
		authz.RepoAction]
}

func (s *SystemUpdaterMockUnauthz) UpdateSystems(ctx context.Context) error {
	s.CallCount += 1
	s.authzLoader.LoadAllowList(ctx)
	_, err := authz.CheckAuthz(ctx, s.authzLoader.AuthzHandler,
		authz.RepoResourceTypeSystemProperty, authz.RepoActionDelete)
	if err != nil {
		return err
	}
	return nil
}

func TestSystemRefresherProcessAction(t *testing.T) {
	numTenants := 3
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{},
		testutils.WithGenerateTenants(numTenants))
	repo := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(),
		repo, &config.Config{})

	authzRepo := authz_repo.NewAuthzRepo(repo, authzRepoLoader)

	t.Run("Should process without error", func(t *testing.T) {
		mock := &SystemUpdaterMock{authzLoader: authzRepoLoader}
		refresher := tasks.NewSystemsRefresher(mock, authzRepo)
		err := refresher.ProcessTask(t.Context(), nil)
		assert.NoError(t, err)
		assert.Equal(t, numTenants, mock.CallCount)
	})

	t.Run("Should error on network error", func(t *testing.T) {
		mock := &SystemUpdaterMockError{}
		refresher := tasks.NewSystemsRefresher(mock, authzRepo)
		err := refresher.ProcessTask(t.Context(), nil)
		assert.ErrorIs(t, err, tasks.ErrRunningTask)
		assert.Equal(t, numTenants, mock.CallCount)
	})

	t.Run("Should error on authz error", func(t *testing.T) {
		mock := &SystemUpdaterMockUnauthz{authzLoader: authzRepoLoader}
		refresher := tasks.NewSystemsRefresher(mock, authzRepo)
		err := refresher.ProcessTask(t.Context(), nil)
		assert.ErrorIs(t, err, authz.ErrAuthorizationDenied)
		assert.Equal(t, numTenants, mock.CallCount)
	})
}
