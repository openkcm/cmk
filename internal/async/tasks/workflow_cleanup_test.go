package tasks_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async/tasks"
	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
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

func TestWorkflowCleanerProcessTask(t *testing.T) {
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	repo := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(),
		repo, &config.Config{})

	authzRepo := authz_repo.NewAuthzRepo(repo, authzRepoLoader)

	t.Run("Should complete successfully", func(t *testing.T) {
		mock := &WorkflowRemovalMock{authzLoader: authzRepoLoader}
		cleaner := tasks.NewWorkflowCleaner(mock, authzRepo)

		err := cleaner.ProcessTask(context.Background(), nil)
		assert.NoError(t, err)
	})

	t.Run("Task type is correct", func(t *testing.T) {
		mock := &WorkflowRemovalMock{authzLoader: authzRepoLoader}
		cleaner := tasks.NewWorkflowCleaner(mock, authzRepo)

		taskType := cleaner.TaskType()
		assert.Equal(t, config.TypeWorkflowCleanup, taskType, "Task type should be WorkflowCleanup")
	})

	t.Run("Should handle nil task parameter", func(t *testing.T) {
		mock := &WorkflowRemovalMock{authzLoader: authzRepoLoader}
		cleaner := tasks.NewWorkflowCleaner(mock, authzRepo)

		err := cleaner.ProcessTask(context.Background(), nil)
		assert.NoError(t, err, "Should handle nil task parameter")
	})

	t.Run("Unauthorized processing", func(t *testing.T) {
		mock := &WorkflowRemovalMockUnauthz{authzLoader: authzRepoLoader}
		cleaner := tasks.NewWorkflowCleaner(mock, authzRepo)
		err := cleaner.ProcessTask(context.Background(), nil)
		assert.ErrorIs(t, err, authz.ErrAuthorizationDenied)
	})
}
