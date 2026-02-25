package noop

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/openkcm/plugin-sdk/pkg/hclog2slog"

	notificationv1 "github.com/openkcm/plugin-sdk/proto/plugin/notification/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
)

func Register(registry catalog.BuiltInPluginRegistry) {
	registry.Register(builtin(NewPlugin()))
}

func builtin(p *Plugin) catalog.BuiltInPlugin {
	return catalog.MakeBuiltIn("noop",
		notificationv1.NotificationServicePluginServer(p),
		configv1.ConfigServiceServer(p))
}

type Plugin struct {
	notificationv1.UnsafeNotificationServiceServer
	configv1.UnsafeConfigServer

	logger    *slog.Logger
	buildInfo string
}

var (
	_ notificationv1.NotificationServiceServer = (*Plugin)(nil)
	_ configv1.ConfigServer                    = (*Plugin)(nil)
)

func NewPlugin() *Plugin {
	return &Plugin{
		buildInfo: "{}",
	}
}

func (p *Plugin) SetLogger(logger hclog.Logger) {
	p.logger = hclog2slog.New(logger)
}

func (p *Plugin) Configure(
	_ context.Context,
	_ *configv1.ConfigureRequest,
) (*configv1.ConfigureResponse, error) {
	slog.Info("Configuring plugin")

	return &configv1.ConfigureResponse{
		BuildInfo: &p.buildInfo,
	}, nil
}

func (p *Plugin) SendNotification(
	_ context.Context,
	_ *notificationv1.SendNotificationRequest,
) (*notificationv1.SendNotificationResponse, error) {
	return &notificationv1.SendNotificationResponse{
		Success: true,
	}, nil
}
