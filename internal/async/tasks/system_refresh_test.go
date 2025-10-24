package tasks_test

import (
	"context"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openkcm/cmk-core/internal/async/tasks"
	"github.com/openkcm/cmk-core/internal/repo/sql"
	"github.com/openkcm/cmk-core/internal/testutils"
)

type SystemUpdaterMock struct{}

//nolint:wrapcheck
func (s *SystemUpdaterMock) UpdateSystems(_ context.Context) error {
	st := status.New(codes.DeadlineExceeded, "timeout")
	return st.Err()
}

func TestSystemRefresherProcessAction(t *testing.T) {
	db, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{&testutils.TestModel{}},
	})
	repo := sql.NewRepository(db)

	refresher := tasks.NewSystemsRefresher(&SystemUpdaterMock{}, repo)

	t.Run("Should error on network error", func(t *testing.T) {
		err := refresher.ProcessTask(t.Context(), nil)
		assert.ErrorIs(t, err, tasks.ErrRunningTask)
	})
}
