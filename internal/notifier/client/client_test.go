package client_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	notificationv1 "github.com/openkcm/plugin-sdk/proto/plugin/notification/v1"

	"github.com/openkcm/cmk/internal/config"
	cmkplugincatalog "github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/notifier/client"
	"github.com/openkcm/cmk/internal/testutils"
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

	t.Run("Success", func(t *testing.T) {
		// Setup
		cfg := config.Config{Plugins: testutils.SetupMockPlugins(testutils.Notification)}
		ctlg, err := cmkplugincatalog.New(t.Context(), &cfg)
		assert.NoError(t, err)

		defer ctlg.Close()

		c := client.New(t.Context(), ctlg)
		c.SetClient(NotificationServiceMock{})
		// Act
		err = c.CreateNotification(t.Context(), msg)
		// Verify
		assert.NoError(t, err)
	})

	t.Run("Failure", func(t *testing.T) {
		// Setup
		cfg := config.Config{Plugins: testutils.SetupMockPlugins(testutils.Notification)}
		ctlg, err := cmkplugincatalog.New(t.Context(), &cfg)
		assert.NoError(t, err)

		defer ctlg.Close()

		c := client.New(t.Context(), ctlg)
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
