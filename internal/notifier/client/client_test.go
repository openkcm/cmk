package client_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	notificationv1 "github.com/openkcm/plugin-sdk/proto/plugin/notification/v1"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/notifier/client"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
)

type NotificationServiceMock struct {
	SendNotificationFunc func(
		ctx context.Context,
		in *notificationv1.SendNotificationRequest,
		opts ...grpc.CallOption) (
		*notificationv1.SendNotificationResponse, error)
}

func (m NotificationServiceMock) SendNotification(
	ctx context.Context,
	in *notificationv1.SendNotificationRequest,
	opts ...grpc.CallOption,
) (*notificationv1.SendNotificationResponse, error) {
	if m.SendNotificationFunc != nil {
		return m.SendNotificationFunc(ctx, in, opts...)
	}

	return &notificationv1.SendNotificationResponse{}, nil
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

		c := client.New(t.Context(), svcRegistry)
		c.SetClient(NotificationServiceMock{})
		// Act
		err = c.CreateNotification(t.Context(), msg)
		// Verify
		assert.NoError(t, err)
	})

	t.Run("Failure", func(t *testing.T) {
		// Setup
		c := client.New(t.Context(), svcRegistry)
		c.SetClient(NotificationServiceMock{
			SendNotificationFunc: func(
				ctx context.Context,
				in *notificationv1.SendNotificationRequest,
				opts ...grpc.CallOption) (
				*notificationv1.SendNotificationResponse, error,
			) {
				return nil, assert.AnError
			},
		})
		// Act
		err = c.CreateNotification(t.Context(), msg)

		// Verify
		assert.Error(t, err)
	})
}
