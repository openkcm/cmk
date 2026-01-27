package tasks_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async/tasks"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

type WorkflowRemovalMock struct{}

func (w *WorkflowRemovalMock) CleanupTerminalWorkflows(_ context.Context) error {
	return nil
}

func TestWorkflowCleanerProcessTask(t *testing.T) {
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	repo := sql.NewRepository(db)

	t.Run("Should complete successfully", func(t *testing.T) {
		mock := &WorkflowRemovalMock{}
		cleaner := tasks.NewWorkflowCleaner(mock, repo)

		err := cleaner.ProcessTask(context.Background(), nil)
		assert.NoError(t, err)
	})

	t.Run("Task type is correct", func(t *testing.T) {
		mock := &WorkflowRemovalMock{}
		cleaner := tasks.NewWorkflowCleaner(mock, repo)

		taskType := cleaner.TaskType()
		assert.Equal(t, config.TypeWorkflowCleanup, taskType, "Task type should be WorkflowCleanup")
	})

	t.Run("Should handle nil task parameter", func(t *testing.T) {
		mock := &WorkflowRemovalMock{}
		cleaner := tasks.NewWorkflowCleaner(mock, repo)

		err := cleaner.ProcessTask(context.Background(), nil)
		assert.NoError(t, err, "Should handle nil task parameter")
	})
}
