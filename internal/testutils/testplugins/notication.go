package testplugins

import (
	"context"
	"log/slog"

	"github.com/openkcm/plugin-sdk/pkg/catalog"

	notificationv1 "github.com/openkcm/plugin-sdk/proto/plugin/notification/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
)

type Notification struct {
	notificationv1.UnimplementedNotificationServiceServer
	configv1.UnsafeConfigServer
}

func NewNotification() catalog.BuiltInPlugin {
	p := &Notification{}
	return catalog.MakeBuiltIn(
		Name,
		notificationv1.NotificationServicePluginServer(p),
		configv1.ConfigServiceServer(p),
	)
}

func (p *Notification) Configure(_ context.Context, _ *configv1.ConfigureRequest) (*configv1.ConfigureResponse, error) {
	slog.Info("Configuring plugin")

	return &configv1.ConfigureResponse{}, nil
}

func (p *Notification) SendNotification(_ context.Context, _ *notificationv1.SendNotificationRequest) (
	*notificationv1.SendNotificationResponse, error,
) {
	return &notificationv1.SendNotificationResponse{}, nil
}
