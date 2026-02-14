package tasks_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/openkcm/plugin-sdk/api"
	"github.com/openkcm/plugin-sdk/api/service/notification"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async/tasks"
	"github.com/openkcm/cmk/internal/config"
)

var ErrMockNotification = errors.New("mock notification error")

// SuccessfulNotifierMock always succeeds
type SuccessfulNotifierMock struct{}

var _ notification.Notification = (*SuccessfulNotifierMock)(nil)

func (s *SuccessfulNotifierMock) ServiceInfo() api.Info {
	panic("implement me")
}

func (s *SuccessfulNotifierMock) Send(_ context.Context, _ *notification.SendNotificationRequest) (*notification.SendNotificationResponse, error) {
	panic("implement me")
}

// FailingNotifierMock always fails with a predefined error
type FailingNotifierMock struct {
	err error
}

func (s *FailingNotifierMock) ServiceInfo() api.Info {
	panic("implement me")
}

func (s *FailingNotifierMock) Send(_ context.Context, _ *notification.SendNotificationRequest) (*notification.SendNotificationResponse, error) {
	panic("implement me")
}

func TestNewNotificationSender(t *testing.T) {
	notifier := &SuccessfulNotifierMock{}
	sender := tasks.NewNotificationSender(notifier)

	assert.NotNil(t, sender, "NotificationSender should not be nil")
}

func TestNotificationSender_TaskType(t *testing.T) {
	// Arrange
	notifier := &SuccessfulNotifierMock{}
	sender := tasks.NewNotificationSender(notifier)

	// Act
	taskType := sender.TaskType()

	// Assert
	assert.Equal(t, config.TypeSendNotifications, taskType)
}

func TestNotificationSender_ProcessTask_Success(t *testing.T) {
	// Arrange
	notifier := &SuccessfulNotifierMock{}
	sender := tasks.NewNotificationSender(notifier)

	notifData := notification.SendNotificationRequest{
		Recipients: []string{"test@example.com", "admin@example.com"},
		Subject:    "Test Notification",
		Body:       "This is a test notification body",
	}

	payload, err := json.Marshal(notifData)
	assert.NoError(t, err)

	task := asynq.NewTask(config.TypeSendNotifications, payload)
	ctx := context.Background()

	// Act
	err = sender.ProcessTask(ctx, task)

	// Assert
	assert.NoError(t, err)
}

func TestNotificationSender_ProcessTask_NotificationFailure(t *testing.T) {
	// Arrange
	expectedErr := ErrMockNotification
	notifier := &FailingNotifierMock{err: expectedErr}
	sender := tasks.NewNotificationSender(notifier)

	notifData := notification.SendNotificationRequest{
		Recipients: []string{"test@example.com"},
		Subject:    "Test Notification",
		Body:       "This is a test notification body",
	}

	payload, err := json.Marshal(notifData)
	assert.NoError(t, err)

	task := asynq.NewTask(config.TypeSendNotifications, payload)
	ctx := context.Background()

	// Act
	err = sender.ProcessTask(ctx, task)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestNotificationSender_ProcessTask_InvalidPayload(t *testing.T) {
	// Arrange
	notifier := &SuccessfulNotifierMock{}
	sender := tasks.NewNotificationSender(notifier)

	invalidPayload := []byte(`{"invalid": json"}`)
	task := asynq.NewTask(config.TypeSendNotifications, invalidPayload)
	ctx := context.Background()

	// Act
	err := sender.ProcessTask(ctx, task)

	// Assert
	assert.Error(t, err, "Should return error for invalid JSON payload")
}

func TestNotificationSender_ProcessTask_EmptyPayload(t *testing.T) {
	// Arrange
	notifier := &SuccessfulNotifierMock{}
	sender := tasks.NewNotificationSender(notifier)

	task := asynq.NewTask(config.TypeSendNotifications, []byte{})
	ctx := context.Background()

	// Act
	err := sender.ProcessTask(ctx, task)

	// Assert
	assert.Error(t, err, "Should return error for empty payload")
}

func TestNotificationSender_ProcessTask_EmptyRecipients(t *testing.T) {
	// Arrange
	notifier := &SuccessfulNotifierMock{}
	sender := tasks.NewNotificationSender(notifier)

	notifData := notification.SendNotificationRequest{
		Recipients: []string{},
		Subject:    "Test Notification",
		Body:       "This is a test notification body",
	}

	payload, err := json.Marshal(notifData)
	assert.NoError(t, err)

	task := asynq.NewTask(config.TypeSendNotifications, payload)
	ctx := context.Background()

	// Act
	err = sender.ProcessTask(ctx, task)

	// Assert
	assert.NoError(t, err)
}

func TestNotificationSender_ProcessTask_SingleRecipient(t *testing.T) {
	// Arrange
	notifier := &SuccessfulNotifierMock{}
	sender := tasks.NewNotificationSender(notifier)

	notifData := notification.SendNotificationRequest{
		Recipients: []string{"single@example.com"},
		Subject:    "Single Recipient Test",
		Body:       "Notification for single recipient",
	}

	payload, err := json.Marshal(notifData)
	assert.NoError(t, err)

	task := asynq.NewTask(config.TypeSendNotifications, payload)
	ctx := context.Background()

	// Act
	err = sender.ProcessTask(ctx, task)

	// Assert
	assert.NoError(t, err)
}
