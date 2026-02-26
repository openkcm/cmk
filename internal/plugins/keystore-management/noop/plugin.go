package noop

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/openkcm/plugin-sdk/pkg/hclog2slog"

	keystoremanagementv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/management/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
)

func Register(registry catalog.BuiltInPluginRegistry) {
	registry.Register(builtin(NewPlugin()))
}

func builtin(p *Plugin) catalog.BuiltInPlugin {
	return catalog.MakeBuiltIn("noop",
		keystoremanagementv1.KeystoreProviderPluginServer(p),
		configv1.ConfigServiceServer(p))
}

type Plugin struct {
	keystoremanagementv1.UnsafeKeystoreProviderServer
	configv1.UnsafeConfigServer

	logger    *slog.Logger
	buildInfo string
}

var (
	_ keystoremanagementv1.UnsafeKeystoreProviderServer = (*Plugin)(nil)
	_ configv1.ConfigServer                             = (*Plugin)(nil)
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

func (p *Plugin) CreateKeystore(
	_ context.Context,
	_ *keystoremanagementv1.CreateKeystoreRequest,
) (*keystoremanagementv1.CreateKeystoreResponse, error) {
	return &keystoremanagementv1.CreateKeystoreResponse{}, nil
}

func (p *Plugin) DeleteKeystore(
	_ context.Context,
	_ *keystoremanagementv1.DeleteKeystoreRequest,
) (*keystoremanagementv1.DeleteKeystoreResponse, error) {
	return &keystoremanagementv1.DeleteKeystoreResponse{}, nil
}
