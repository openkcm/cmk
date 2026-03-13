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
	repo := sql.NewRepository(db)

	refresher := tasks.NewSystemsRefresher(&SystemUpdaterMock{}, repo)

	task := asynq.NewTask(config.TypeSystemsTask, nil)

	t.Run("Should error on network error", func(t *testing.T) {
		err := refresher.ProcessTask(t.Context(), task)
		assert.Error(t, err)
	})
}
