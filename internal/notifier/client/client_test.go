package client_test

import (
	"context"
	"testing"

	"github.com/openkcm/plugin-sdk/api"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/notifier/client"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/notification"
	"github.com/openkcm/cmk/internal/testutils"
)

type NotificationMock struct {
	SendNotificationFunc func(
		ctx context.Context,
		req *notification.SendNotificationRequest,
	) (*notification.SendNotificationResponse, error)
}

func (m NotificationMock) ServiceInfo() api.Info {
	panic("implement me")
}

func (m NotificationMock) Send(
	ctx context.Context,
	req *notification.SendNotificationRequest,
) (*notification.SendNotificationResponse, error) {
	if m.SendNotificationFunc != nil {
		return m.SendNotificationFunc(ctx, req)
	}

	return &notification.SendNotificationResponse{}, nil
}

func TestCreateNotificationManager(t *testing.T) {
	msg := client.Data{
		Recipients: []string{"user1"},
		Subject:    "Test Notification",
		Body:       "Test Notification",
	}

	svcRegistry := testutils.NewTestPlugins()
	defer svcRegistry.Close()

	t.Run("Success", func(t *testing.T) {
		c, err := client.New(t.Context(), svcRegistry)
		assert.NoError(t, err)
		c.SetService(NotificationMock{})
		err = c.CreateNotification(t.Context(), msg)
		assert.NoError(t, err)
	})

	t.Run("Failure", func(t *testing.T) {
		c, err := client.New(t.Context(), svcRegistry)
		assert.NoError(t, err)
		c.SetService(NotificationMock{
			SendNotificationFunc: func(
				ctx context.Context,
				req *notification.SendNotificationRequest,
			) (*notification.SendNotificationResponse, error) {
				return nil, assert.AnError
			},
		})
		err = c.CreateNotification(t.Context(), msg)
		assert.Error(t, err)
	})
}
