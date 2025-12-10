package tasks_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/async/tasks"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/repo/sql"
	"github.tools.sap/kms/cmk/internal/testutils"
)

var errMockSyncHYOKClient = errors.New("error syncing hyok client")

type HyokHYOKClientMock struct{}

func (s *HyokHYOKClientMock) SyncHYOKKeys(_ context.Context) error {
	return nil
}

type HyokHYOKClientMockFailed struct{}

func (s *HyokHYOKClientMockFailed) SyncHYOKKeys(_ context.Context) error {
	return errMockSyncHYOKClient
}

func TestHYOKSyncProcessAction(t *testing.T) {
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{&testutils.TestModel{}},
	})
	repo := sql.NewRepository(db)
	sync := tasks.NewHYOKSync(&HyokHYOKClientMock{}, repo)

	t.Run("Should complete", func(t *testing.T) {
		err := sync.ProcessTask(t.Context(), nil)
		assert.NoError(t, err)
	})

	t.Run("Task type is right", func(t *testing.T) {
		taskType := sync.TaskType()
		assert.Equal(t, config.TypeHYOKSync, taskType, "Task type should be HYOKSync")
	})

	t.Run("Task continues one failure of hyok client", func(t *testing.T) {
		sync := tasks.NewHYOKSync(&HyokHYOKClientMockFailed{}, repo)
		err := sync.ProcessTask(t.Context(), nil)
		assert.NoError(t, err)
	})
}
