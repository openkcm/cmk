package tasks_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/async/tasks"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	asyncUtils "github.com/openkcm/cmk/utils/async"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

var errMockUpdateSystem = errors.New("update system error")

type systemRoleUpdaterMock struct {
	updateErr  error
	empty      bool
	emptyErr   error
	updateCall int
}

func (m *systemRoleUpdaterMock) UpdateSystemByExternalID(_ context.Context, _ string) error {
	m.updateCall++
	return m.updateErr
}

func (m *systemRoleUpdaterMock) HasEmptyRoleByExternalID(_ context.Context, _ string) (bool, error) {
	return m.empty, m.emptyErr
}

func TestSystemRoleBackfill(t *testing.T) {
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)

	ctx, err := cmkcontext.InjectInternalUserData(t.Context(), constants.InternalTaskSystemRoleBackfillRole)
	require.NoError(t, err)

	const externalID = "788a9823-d7f2-446e-8623-105023dee756"

	makeValidTask := func(id string) *asynq.Task {
		payload := asyncUtils.NewTaskPayload(ctx, []byte(id))
		payloadBytes, marshalErr := payload.ToBytes()
		require.NoError(t, marshalErr)
		return asynq.NewTask(config.TypeSystemRoleBackfill, payloadBytes)
	}

	t.Run("should return error on unparseable payload", func(t *testing.T) {
		handler := tasks.NewSystemRoleBackfill(&systemRoleUpdaterMock{}, r)

		task := asynq.NewTask(config.TypeSystemRoleBackfill, []byte("not-valid-json"))
		err := handler.ProcessTask(ctx, task)

		assert.Error(t, err)
	})

	t.Run("should return error when enrichment fails", func(t *testing.T) {
		mock := &systemRoleUpdaterMock{updateErr: errMockUpdateSystem}
		handler := tasks.NewSystemRoleBackfill(mock, r)

		err := handler.ProcessTask(ctx, makeValidTask(externalID))

		assert.ErrorIs(t, err, errMockUpdateSystem)
	})

	t.Run("should retry when role is still empty", func(t *testing.T) {
		mock := &systemRoleUpdaterMock{empty: true}
		handler := tasks.NewSystemRoleBackfill(mock, r)

		err := handler.ProcessTask(ctx, makeValidTask(externalID))

		assert.ErrorIs(t, err, tasks.ErrRoleStillEmpty)
		assert.Equal(t, 1, mock.updateCall)
	})

	t.Run("should return the emptiness-check error", func(t *testing.T) {
		mock := &systemRoleUpdaterMock{emptyErr: errMockUpdateSystem}
		handler := tasks.NewSystemRoleBackfill(mock, r)

		err := handler.ProcessTask(ctx, makeValidTask(externalID))

		assert.ErrorIs(t, err, errMockUpdateSystem)
	})

	t.Run("should return nil when role is populated", func(t *testing.T) {
		mock := &systemRoleUpdaterMock{empty: false}
		handler := tasks.NewSystemRoleBackfill(mock, r)

		err := handler.ProcessTask(ctx, makeValidTask(externalID))

		assert.NoError(t, err)
	})

	t.Run("should return correct task type", func(t *testing.T) {
		handler := tasks.NewSystemRoleBackfill(&systemRoleUpdaterMock{}, r)

		assert.Equal(t, config.TypeSystemRoleBackfill, handler.TaskType())
	})

	t.Run("should return correct role", func(t *testing.T) {
		handler := tasks.NewSystemRoleBackfill(&systemRoleUpdaterMock{}, r)

		assert.Equal(t, constants.InternalTaskSystemRoleBackfillRole, handler.Role())
	})
}
