package tasks_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tasks "github.com/openkcm/cmk/internal/async/tasks/tenant"
	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/repo"
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

var errMockSystemRefresh = errors.New("system refresh mock failed")

type SystemUpdaterMockNonConnection struct{}

func (s *SystemUpdaterMockNonConnection) UpdateSystems(_ context.Context) error {
	return errMockSystemRefresh
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
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(),
		r, &config.Config{})

	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	mock := &SystemUpdaterMock{authzLoader: authzRepoLoader}
	refresher := tasks.NewSystemsRefresher(mock, authzRepo)

	task := asynq.NewTask(config.TypeSystemsTask, nil)

	t.Run("Should process without error", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		err := refresher.ProcessTask(t.Context(), task)
		assert.NoError(t, err)
		assert.NotContains(t, strings.ToLower(buf.String()), "error")
	})

	t.Run("Should have right taskType", func(t *testing.T) {
		assert.Equal(t, config.TypeSystemsTask, refresher.TaskType())
	})

	t.Run("Should have default tenant query", func(t *testing.T) {
		assert.Equal(t, repo.NewQuery(), refresher.TenantQuery())
	})

	t.Run("Should log error on non-connection error", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		nonConnRefresher := tasks.NewSystemsRefresher(
			&SystemUpdaterMockNonConnection{}, authzRepo)
		err := nonConnRefresher.ProcessTask(t.Context(), task)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Error during system refresh batch processing")
		assert.Contains(t, buf.String(), "system refresh mock failed")
	})

	t.Run("Should log on unauthorized processing", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		mock := &SystemUpdaterMockUnauthz{authzLoader: authzRepoLoader}
		refresher := tasks.NewSystemsRefresher(mock, authzRepo)
		err := refresher.ProcessTask(t.Context(), task)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Error during system refresh batch processing")
		assert.Contains(t, buf.String(), "authorization decision error")
	})
}
