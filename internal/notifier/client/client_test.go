package client_test

import (
	"context"
	"testing"

	"github.com/openkcm/plugin-sdk/api"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/notifier/client"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/notification"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
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

	ps, psCfg := testutils.NewTestPlugins(testplugins.NewNotification())
	cfg := config.Config{Plugins: psCfg}
	svcRegistry, err := cmkpluginregistry.New(t.Context(), &cfg, cmkpluginregistry.WithBuiltInPlugins(ps))
	assert.NoError(t, err)

	defer svcRegistry.Close()

	t.Run("Success", func(t *testing.T) {
		// Setup

		c, err := client.New(t.Context(), svcRegistry)
		assert.NoError(t, err)
		c.SetService(NotificationMock{})
		// Act
		err = c.CreateNotification(t.Context(), msg)
		// Verify
		assert.NoError(t, err)
	})

	t.Run("Failure", func(t *testing.T) {
		// Setup
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
		// Act
		err = c.CreateNotification(t.Context(), msg)

		// Verify
		assert.Error(t, err)
	})
}
