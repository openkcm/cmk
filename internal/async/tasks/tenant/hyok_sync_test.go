package tasks_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	tasks "github.com/openkcm/cmk/internal/async/tasks/tenant"
	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

var errMockSyncHYOKClient = errors.New("error syncing hyok client")

var allowedHYOKTestActions = []authz.RepoAction{
	authz.RepoActionList,
	authz.RepoActionCount,
	authz.RepoActionUpdate,
}

type HYOKClientMock struct {
	CallCount   int
	authzLoader *authz_loader.AuthzLoader[authz.RepoResourceTypeName,
		authz.RepoAction]
}

func (s *HYOKClientMock) SyncHYOKKeys(ctx context.Context) error {
	s.authzLoader.LoadAllowList(ctx)
	for _, testAction := range allowedHYOKTestActions {
		isAllowed, err := authz.CheckAuthz(ctx, s.authzLoader.AuthzHandler,
			authz.RepoResourceTypeKey, testAction)
		if err != nil {
			return err
		}
		if !isAllowed {
			return authz.ErrAuthzDecision
		}
	}
	return nil
}

type HYOKClientMockFailed struct {
	CallCount int
}

func (s *HYOKClientMockFailed) SyncHYOKKeys(_ context.Context) error {
	s.CallCount += 1
	return errMockSyncHYOKClient
}

type HYOKClientMockUnauthz struct {
	CallCount   int
	authzLoader *authz_loader.AuthzLoader[authz.RepoResourceTypeName,
		authz.RepoAction]
}

func (s *HYOKClientMockUnauthz) SyncHYOKKeys(ctx context.Context) error {
	s.CallCount += 1
	s.authzLoader.LoadAllowList(ctx)
	_, err := authz.CheckAuthz(ctx, s.authzLoader.AuthzHandler,
		authz.RepoResourceTypeKey, authz.RepoActionDelete)
	if err != nil {
		return err
	}
	return nil
}

func TestHYOKSyncProcessAction(t *testing.T) {
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(),
		r, &config.Config{})

	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	mock := &HYOKClientMock{authzLoader: authzRepoLoader}
	sync := tasks.NewHYOKSync(mock, authzRepo)

	task := asynq.NewTask(config.TypeHYOKSync, nil)

	t.Run("Should process without error", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		err := sync.ProcessTask(t.Context(), task)
		assert.NoError(t, err)
		assert.NotContains(t, strings.ToLower(buf.String()), "error")
	})

	t.Run("Should have right taskType", func(t *testing.T) {
		assert.Equal(t, config.TypeHYOKSync, sync.TaskType())
	})

	t.Run("Should have default tenant query", func(t *testing.T) {
		assert.Equal(t, repo.NewQuery(), sync.TenantQuery())
	})

	t.Run("Should log error on task failure", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		failSync := tasks.NewHYOKSync(&HYOKClientMockFailed{}, r)
		err := failSync.ProcessTask(t.Context(), task)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Error during hyok sync batch processing")
		assert.Contains(t, buf.String(), "error syncing hyok client")
	})

	t.Run("Should log on unauthorized processing", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		mock := &HYOKClientMockUnauthz{authzLoader: authzRepoLoader}
		sync := tasks.NewHYOKSync(mock, authzRepo)
		err := sync.ProcessTask(t.Context(), task)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Error during hyok sync batch processing")
		assert.Contains(t, buf.String(), "authorization decision error")
	})
}
