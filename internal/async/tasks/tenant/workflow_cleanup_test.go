package tasks_test

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	tasks "github.com/openkcm/cmk/internal/async/tasks/tenant"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

type WorkflowRemovalMock struct{}

func (w *WorkflowRemovalMock) CleanupTerminalWorkflows(_ context.Context) error {
	return nil
}

func TestWorkflowCleanerProcessTask(t *testing.T) {
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)

	task := asynq.NewTask(config.TypeWorkflowCleanup, nil)
	mock := &WorkflowRemovalMock{}
	cleaner := tasks.NewWorkflowCleaner(mock, r)

	t.Run("Should complete successfully", func(t *testing.T) {
		err := cleaner.ProcessTask(context.Background(), task)
		assert.NoError(t, err)
	})

	t.Run("Task type is correct", func(t *testing.T) {
		taskType := cleaner.TaskType()
		assert.Equal(t, config.TypeWorkflowCleanup, taskType, "Task type should be WorkflowCleanup")
	})

	t.Run("Should handle nil task parameter", func(t *testing.T) {
		err := cleaner.ProcessTask(context.Background(), task)
		assert.NoError(t, err, "Should handle nil task parameter")
	})

	t.Run("Should have right taskType", func(t *testing.T) {
		assert.Equal(t, config.TypeWorkflowCleanup, cleaner.TaskType())
	})

	t.Run("Should have default tenant query", func(t *testing.T) {
		assert.Equal(t, repo.NewQuery(), cleaner.TenantQuery())
	})
}
