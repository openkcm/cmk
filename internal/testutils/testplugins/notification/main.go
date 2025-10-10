package main

import (
	"context"

	"github.com/openkcm/plugin-sdk/pkg/plugin"

	notificationv1 "github.com/openkcm/plugin-sdk/proto/plugin/notification/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
)

type TestPlugin struct {
	configv1.UnsafeConfigServer
	notificationv1.UnimplementedNotificationServiceServer
}

var _ notificationv1.NotificationServiceServer = (*TestPlugin)(nil)

func (p *TestPlugin) Configure(_ context.Context, _ *configv1.ConfigureRequest) (*configv1.ConfigureResponse, error) {
	return &configv1.ConfigureResponse{}, nil
}

func (p *TestPlugin) SendNotification(_ context.Context, _ *notificationv1.SendNotificationRequest) (
	*notificationv1.SendNotificationResponse, error,
) {
	return &notificationv1.SendNotificationResponse{}, nil
}

func New() *TestPlugin {
	return &TestPlugin{}
}

func main() {
	server := New()

	plugin.Serve(notificationv1.NotificationServicePluginServer(server))
}
