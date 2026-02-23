package noop

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/openkcm/plugin-sdk/pkg/hclog2slog"

	systeminformationv1 "github.com/openkcm/plugin-sdk/proto/plugin/systeminformation/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
)

func Register(registry catalog.BuiltInPluginRegistry) {
	registry.Register(builtin(NewPlugin()))
}

func builtin(p *Plugin) catalog.BuiltInPlugin {
	return catalog.MakeBuiltIn("noop",
		systeminformationv1.SystemInformationServicePluginServer(p),
		configv1.ConfigServiceServer(p))
}

// Plugin is a simple test implementation of KeystoreProviderServer
type Plugin struct {
	systeminformationv1.UnsafeSystemInformationServiceServer
	configv1.UnsafeConfigServer

	logger    *slog.Logger
	buildInfo string
}

var (
	_ systeminformationv1.SystemInformationServiceServer = (*Plugin)(nil)
	_ configv1.ConfigServer                              = (*Plugin)(nil)
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

func (p *Plugin) Get(
	_ context.Context,
	_ *systeminformationv1.GetRequest,
) (*systeminformationv1.GetResponse, error) {
	return &systeminformationv1.GetResponse{}, nil
}
