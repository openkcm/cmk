package testplugins

import (
	"context"

	"github.com/openkcm/plugin-sdk/api"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/notification"
	servicewrapper "github.com/openkcm/cmk/internal/pluginregistry/service/wrapper"
)

type TestNotification struct{}

var _ notification.Notification = (*TestNotification)(nil)

func NewTestNotification() *TestNotification {
	return &TestNotification{}
}

func (s *TestNotification) ServiceInfo() api.Info {
	return testInfo{
		configuredType: servicewrapper.NotificationServiceType,
	}
}

func (s *TestNotification) Send(
	_ context.Context,
	_ *notification.SendNotificationRequest,
) (*notification.SendNotificationResponse, error) {
	return &notification.SendNotificationResponse{}, nil
}
