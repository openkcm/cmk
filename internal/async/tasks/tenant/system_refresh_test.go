package tasks_test

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tasks "github.com/openkcm/cmk/internal/async/tasks/tenant"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

type SystemUpdaterMock struct{}

//nolint:wrapcheck
func (s *SystemUpdaterMock) UpdateSystems(_ context.Context) error {
	st := status.New(codes.DeadlineExceeded, "timeout")
	return st.Err()
}

func TestSystemRefresherProcessAction(t *testing.T) {
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)

	refresher := tasks.NewSystemsRefresher(&SystemUpdaterMock{}, r)

	task := asynq.NewTask(config.TypeSystemsTask, nil)

	t.Run("Should error on network error", func(t *testing.T) {
		err := refresher.ProcessTask(t.Context(), task)
		assert.Error(t, err)
	})

	t.Run("Should have right taskType", func(t *testing.T) {
		assert.Equal(t, config.TypeSystemsTask, refresher.TaskType())
	})

	t.Run("Should have default tenant query", func(t *testing.T) {
		assert.Equal(t, repo.NewQuery(), refresher.TenantQuery())
	})
}
