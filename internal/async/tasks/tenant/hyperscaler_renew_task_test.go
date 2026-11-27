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
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/testutils"
)

var errMockRenewHyperscaler = errors.New("renew boom")

type hyperscalerRenewerMock struct {
	err error
}

func (m *hyperscalerRenewerMock) RenewHyperscalerLease(context.Context) error {
	return m.err
}

func TestHyperscalerRenew(t *testing.T) {
	task := asynq.NewTask(config.TypeHyperscalerRenew, nil)

	t.Run("exposes the task type, role, fan-out and tenant query", func(t *testing.T) {
		renew := tasks.NewHyperscalerRenew(&hyperscalerRenewerMock{}, nil)
		assert.Equal(t, config.TypeHyperscalerRenew, renew.TaskType())
		assert.Equal(t, constants.InternalTaskHyperscalerRenewRole, renew.Role())
		assert.NotNil(t, renew.FanOutFunc())
		assert.Equal(t, repo.NewQuery(), renew.TenantQuery())
	})

	t.Run("processes without error", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		renew := tasks.NewHyperscalerRenew(&hyperscalerRenewerMock{}, nil)
		assert.NoError(t, renew.ProcessTask(t.Context(), task))
		assert.NotContains(t, strings.ToLower(buf.String()), "error")
	})

	t.Run("logs and swallows a per-tenant renewal error", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		renew := tasks.NewHyperscalerRenew(&hyperscalerRenewerMock{err: errMockRenewHyperscaler}, nil)
		assert.NoError(t, renew.ProcessTask(t.Context(), task))
		assert.Contains(t, buf.String(), "Error during hyperscaler renewal batch processing")
		assert.Contains(t, buf.String(), "renew boom")
	})
}
