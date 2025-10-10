package management

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/openkcm/plugin-sdk/pkg/hclog2slog"

	managementv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/management/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
	slogctx "github.com/veqryn/slog-context"
)

const (
	pluginName = "keystore-management-empty"
)

func V1BuiltIn() catalog.BuiltIn {
	return builtin(&V1Plugin{})
}

func builtin(p *V1Plugin) catalog.BuiltIn {
	return catalog.MakeBuiltIn(pluginName,
		managementv1.KeystoreProviderPluginServer(p),
		configv1.ConfigServiceServer(p))
}

type V1Plugin struct {
	configv1.UnsafeConfigServer
	managementv1.KeystoreProviderServer
}

var (
	_ managementv1.KeystoreProviderServer = (*V1Plugin)(nil)
	_ configv1.ConfigServer               = (*V1Plugin)(nil)
)

// SetLogger method is called whenever the plugin start and giving the logger of host application
func (p *V1Plugin) SetLogger(logger hclog.Logger) {
	slog.SetDefault(hclog2slog.New(logger))
}

// Configure configures the plugin with the given configuration
func (p *V1Plugin) Configure(
	ctx context.Context,
	_ *configv1.ConfigureRequest,
) (*configv1.ConfigureResponse, error) {
	slogctx.Debug(ctx, "Builtin System Information Service (SIS) plugin")

	return &configv1.ConfigureResponse{}, nil
}

func (p *V1Plugin) CreateKeystore(
	ctx context.Context,
	_ *managementv1.CreateKeystoreRequest,
) (*managementv1.CreateKeystoreResponse, error) {
	slogctx.Debug(ctx, "Builtin Key Store Management Service (KSM) - CreateKeystore called")

	return &managementv1.CreateKeystoreResponse{}, nil
}
func (p *V1Plugin) DeleteKeystore(
	ctx context.Context,
	_ *managementv1.DeleteKeystoreRequest,
) (*managementv1.DeleteKeystoreResponse, error) {
	slogctx.Debug(ctx, "Builtin Key Store Management Service (KSM) - DeleteKeystore called")

	return &managementv1.DeleteKeystoreResponse{}, nil
}
