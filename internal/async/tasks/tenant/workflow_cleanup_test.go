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

var allowedWorkflowCleanupTestActions = []authz.RepoAction{
	authz.RepoActionCount,
	authz.RepoActionList,
	authz.RepoActionDelete,
}

type WorkflowRemovalMock struct {
	authzLoader *authz_loader.AuthzLoader[authz.RepoResourceTypeName,
		authz.RepoAction]
}

func (w *WorkflowRemovalMock) CleanupTerminalWorkflows(ctx context.Context) error {
	for _, testAction := range allowedWorkflowCleanupTestActions {
		isAllowed, err := authz.CheckAuthz(ctx, w.authzLoader.AuthzHandler,
			authz.RepoResourceTypeWorkflow, testAction)
		if err != nil {
			return err
		}
		if !isAllowed {
			return authz.ErrAuthzDecision
		}
	}
	return nil
}

type WorkflowRemovalMockUnauthz struct {
	authzLoader *authz_loader.AuthzLoader[authz.RepoResourceTypeName,
		authz.RepoAction]
}

func (w *WorkflowRemovalMockUnauthz) CleanupTerminalWorkflows(ctx context.Context) error {
	isAllowed, err := authz.CheckAuthz(ctx, w.authzLoader.AuthzHandler,
		authz.RepoResourceTypeSystem, authz.RepoActionUpdate)
	if err != nil {
		return err
	}
	if !isAllowed {
		return authz.ErrAuthzDecision
	}
	return nil
}

var errMockCleanupFailed = errors.New("mock cleanup failed")

type WorkflowRemovalMockFailed struct{}

func (w *WorkflowRemovalMockFailed) CleanupTerminalWorkflows(_ context.Context) error {
	return errMockCleanupFailed
}

func TestWorkflowCleanerProcessTask(t *testing.T) {
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(),
		r, &config.Config{})

	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	mock := &WorkflowRemovalMock{authzLoader: authzRepoLoader}
	cleaner := tasks.NewWorkflowCleaner(mock, authzRepo)

	task := asynq.NewTask(config.TypeWorkflowCleanup, nil)

	t.Run("Should complete successfully", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		err := cleaner.ProcessTask(context.Background(), task)
		assert.NoError(t, err)
		assert.NotContains(t, strings.ToLower(buf.String()), "error")
	})

	t.Run("Should have right taskType", func(t *testing.T) {
		assert.Equal(t, config.TypeWorkflowCleanup, cleaner.TaskType())
	})

	t.Run("Should have default tenant query", func(t *testing.T) {
		assert.Equal(t, repo.NewQuery(), cleaner.TenantQuery())
	})

	t.Run("Should handle nil task parameter", func(t *testing.T) {
		mock := &WorkflowRemovalMock{authzLoader: authzRepoLoader}
		cleaner := tasks.NewWorkflowCleaner(mock, authzRepo)

		err := cleaner.ProcessTask(context.Background(), nil)
		assert.NoError(t, err, "Should handle nil task parameter")
	})

	t.Run("Should log on unauthorized processing", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		mock := &WorkflowRemovalMockUnauthz{authzLoader: authzRepoLoader}
		cleaner := tasks.NewWorkflowCleaner(mock, authzRepo)
		err := cleaner.ProcessTask(context.Background(), task)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Error during workflow cleanup batch processing")
		assert.Contains(t, buf.String(), "authorization decision error")
	})

	t.Run("Should log error on task failure", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		failCleaner := tasks.NewWorkflowCleaner(&WorkflowRemovalMockFailed{}, r)
		err := failCleaner.ProcessTask(context.Background(), task)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Error during workflow cleanup batch processing")
		assert.Contains(t, buf.String(), "mock cleanup failed")
	})
}
