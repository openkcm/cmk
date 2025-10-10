package notification

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/openkcm/plugin-sdk/pkg/hclog2slog"
	notificationv1 "github.com/openkcm/plugin-sdk/proto/plugin/notification/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
	slogctx "github.com/veqryn/slog-context"
)

const (
	pluginName = "notification-empty"
)

func V1BuiltIn() catalog.BuiltIn {
	return builtin(&V1Plugin{})
}

func builtin(p *V1Plugin) catalog.BuiltIn {
	return catalog.MakeBuiltIn(pluginName,
		notificationv1.NotificationServicePluginServer(p),
		configv1.ConfigServiceServer(p))
}

type V1Plugin struct {
	configv1.UnsafeConfigServer
	notificationv1.NotificationServiceServer
}

var (
	_ notificationv1.NotificationServiceServer = (*V1Plugin)(nil)
	_ configv1.ConfigServer                    = (*V1Plugin)(nil)
)

// SetLogger method is called whenever the plugin start and giving the logger of host application
func (p *V1Plugin) SetLogger(logger hclog.Logger) {
	slog.SetDefault(hclog2slog.New(logger))
}

// Configure configures the plugin with the given configuration
func (p *V1Plugin) Configure(ctx context.Context, _ *configv1.ConfigureRequest) (*configv1.ConfigureResponse, error) {
	slogctx.Debug(ctx, "Builtin System Information Service (SIS) plugin")

	return &configv1.ConfigureResponse{}, nil
}

func (p *V1Plugin) SendNotification(context.Context, *notificationv1.SendNotificationRequest) (*notificationv1.SendNotificationResponse, error) {
	slogctx.Debug(context.Background(), "Builtin Notification Service (NS) - SendNotification called")

	return &notificationv1.SendNotificationResponse{}, nil
}
