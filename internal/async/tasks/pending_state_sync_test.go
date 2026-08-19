package tasks_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
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

var errMockSyncPendingKey = errors.New("sync pending creation key error")

type pendingStateUpdaterMock struct {
	err error
}

func (m *pendingStateUpdaterMock) SyncPendingCreationKey(_ context.Context, _ uuid.UUID) error {
	return m.err
}

func (m *pendingStateUpdaterMock) SyncPendingRegistrationKey(_ context.Context, _ uuid.UUID) error {
	return m.err
}

func TestPendingStateSync(t *testing.T) {
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)

	ctx, err := cmkcontext.InjectInternalUserData(t.Context(), constants.InternalTaskPendingStateSyncRole)
	require.NoError(t, err)

	keyID := uuid.New()

	// Helper: build a valid task payload containing the given UUID.
	makeValidTask := func(id uuid.UUID) *asynq.Task {
		payload := asyncUtils.NewTaskPayload(ctx, []byte(id.String()))
		payloadBytes, marshalErr := payload.ToBytes()
		require.NoError(t, marshalErr)
		return asynq.NewTask(config.TypePendingStateSync, payloadBytes)
	}

	t.Run("should return error on unparseable payload", func(t *testing.T) {
		mock := &pendingStateUpdaterMock{}
		handler := tasks.NewPendingStateSync(mock, r)

		task := asynq.NewTask(config.TypePendingStateSync, []byte("not-valid-json"))
		err := handler.ProcessTask(ctx, task)

		assert.Error(t, err)
	})

	t.Run("should return error on non-UUID payload data", func(t *testing.T) {
		mock := &pendingStateUpdaterMock{}
		handler := tasks.NewPendingStateSync(mock, r)

		payload := asyncUtils.NewTaskPayload(ctx, []byte("not-a-uuid"))
		payloadBytes, marshalErr := payload.ToBytes()
		require.NoError(t, marshalErr)

		task := asynq.NewTask(config.TypePendingStateSync, payloadBytes)
		err := handler.ProcessTask(ctx, task)

		assert.Error(t, err)
	})

	t.Run("should return error when sync fails", func(t *testing.T) {
		mock := &pendingStateUpdaterMock{err: errMockSyncPendingKey}
		handler := tasks.NewPendingStateSync(mock, r)

		err := handler.ProcessTask(ctx, makeValidTask(keyID))

		assert.ErrorIs(t, err, errMockSyncPendingKey)
	})

	t.Run("should return nil on success", func(t *testing.T) {
		mock := &pendingStateUpdaterMock{}
		handler := tasks.NewPendingStateSync(mock, r)

		err := handler.ProcessTask(ctx, makeValidTask(keyID))

		assert.NoError(t, err)
	})

	t.Run("should return correct task type", func(t *testing.T) {
		mock := &pendingStateUpdaterMock{}
		handler := tasks.NewPendingStateSync(mock, r)

		assert.Equal(t, config.TypePendingStateSync, handler.TaskType())
	})

	t.Run("should return correct role", func(t *testing.T) {
		mock := &pendingStateUpdaterMock{}
		handler := tasks.NewPendingStateSync(mock, r)

		assert.Equal(t, constants.InternalTaskPendingStateSyncRole, handler.Role())
	})
}
