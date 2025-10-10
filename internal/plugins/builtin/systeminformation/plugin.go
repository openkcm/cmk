package systeminformation

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/openkcm/plugin-sdk/pkg/hclog2slog"
	systeminformationv1 "github.com/openkcm/plugin-sdk/proto/plugin/systeminformation/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
	slogctx "github.com/veqryn/slog-context"
)

const (
	pluginName = "system-information-empty"
)

func V1BuiltIn() catalog.BuiltIn {
	return builtin(NewV1Plugin())
}

func builtin(p *V1Plugin) catalog.BuiltIn {
	return catalog.MakeBuiltIn(pluginName,
		systeminformationv1.SystemInformationServicePluginServer(p),
		configv1.ConfigServiceServer(p))
}

type V1Plugin struct {
	configv1.UnsafeConfigServer
	systeminformationv1.UnimplementedSystemInformationServiceServer
}

var (
	_ systeminformationv1.SystemInformationServiceServer = (*V1Plugin)(nil)
	_ configv1.ConfigServer                              = (*V1Plugin)(nil)
)

func NewV1Plugin() *V1Plugin {
	return &V1Plugin{}
}

// SetLogger method is called whenever the plugin start and giving the logger of host application
func (p *V1Plugin) SetLogger(logger hclog.Logger) {
	slog.SetDefault(hclog2slog.New(logger))
}

// Configure configures the plugin with the given configuration
func (p *V1Plugin) Configure(ctx context.Context, _ *configv1.ConfigureRequest) (*configv1.ConfigureResponse, error) {
	slogctx.Debug(ctx, "Builtin System Information Service (SIS) plugin")

	return &configv1.ConfigureResponse{}, nil
}

// Get V1Plugin method/operation
func (p *V1Plugin) Get(ctx context.Context, req *systeminformationv1.GetRequest) (*systeminformationv1.GetResponse, error) {

	slogctx.Debug(ctx, "Builtin System Information Service (SIS) - Get called", "req", req.GetId())

	return &systeminformationv1.GetResponse{}, nil
}
