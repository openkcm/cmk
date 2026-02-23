package noop

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/openkcm/plugin-sdk/pkg/hclog2slog"

	keymanagementv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
)

func Register(registry catalog.BuiltInPluginRegistry) {
	registry.Register(builtin(NewPlugin()))
}

func builtin(p *Plugin) catalog.BuiltInPlugin {
	return catalog.MakeBuiltIn("noop",
		keymanagementv1.KeystoreInstanceKeyOperationPluginServer(p),
		configv1.ConfigServiceServer(p))
}

// Plugin is a simple test implementation of KeystoreProviderServer
type Plugin struct {
	keymanagementv1.UnsafeKeystoreInstanceKeyOperationServer
	configv1.UnsafeConfigServer

	logger    *slog.Logger
	buildInfo string
}

var (
	_ keymanagementv1.UnsafeKeystoreInstanceKeyOperationServer = (*Plugin)(nil)
	_ configv1.ConfigServer                                    = (*Plugin)(nil)
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

func (p *Plugin) GetKey(
	_ context.Context,
	_ *keymanagementv1.GetKeyRequest,
) (*keymanagementv1.GetKeyResponse, error) {
	return &keymanagementv1.GetKeyResponse{}, nil
}

func (p *Plugin) CreateKey(
	_ context.Context,
	_ *keymanagementv1.CreateKeyRequest,
) (*keymanagementv1.CreateKeyResponse, error) {
	return &keymanagementv1.CreateKeyResponse{}, nil
}

func (p *Plugin) DeleteKey(
	_ context.Context,
	_ *keymanagementv1.DeleteKeyRequest,
) (*keymanagementv1.DeleteKeyResponse, error) {
	return &keymanagementv1.DeleteKeyResponse{}, nil
}

func (p *Plugin) EnableKey(
	_ context.Context,
	_ *keymanagementv1.EnableKeyRequest,
) (*keymanagementv1.EnableKeyResponse, error) {
	return &keymanagementv1.EnableKeyResponse{}, nil
}

func (p *Plugin) DisableKey(
	_ context.Context,
	_ *keymanagementv1.DisableKeyRequest,
) (*keymanagementv1.DisableKeyResponse, error) {
	return &keymanagementv1.DisableKeyResponse{}, nil
}

func (p *Plugin) GetImportParameters(
	_ context.Context,
	_ *keymanagementv1.GetImportParametersRequest,
) (*keymanagementv1.GetImportParametersResponse, error) {
	return &keymanagementv1.GetImportParametersResponse{}, nil
}

func (p *Plugin) ImportKeyMaterial(
	_ context.Context,
	_ *keymanagementv1.ImportKeyMaterialRequest,
) (*keymanagementv1.ImportKeyMaterialResponse, error) {
	return &keymanagementv1.ImportKeyMaterialResponse{}, nil
}

func (p *Plugin) ValidateKey(
	_ context.Context,
	_ *keymanagementv1.ValidateKeyRequest,
) (*keymanagementv1.ValidateKeyResponse, error) {
	return &keymanagementv1.ValidateKeyResponse{}, nil
}

func (p *Plugin) ValidateKeyAccessData(
	_ context.Context,
	_ *keymanagementv1.ValidateKeyAccessDataRequest,
) (*keymanagementv1.ValidateKeyAccessDataResponse, error) {
	return &keymanagementv1.ValidateKeyAccessDataResponse{}, nil
}

func (p *Plugin) TransformCryptoAccessData(
	_ context.Context,
	_ *keymanagementv1.TransformCryptoAccessDataRequest,
) (*keymanagementv1.TransformCryptoAccessDataResponse, error) {
	return &keymanagementv1.TransformCryptoAccessDataResponse{}, nil
}

func (p *Plugin) ExtractKeyRegion(
	_ context.Context,
	_ *keymanagementv1.ExtractKeyRegionRequest,
) (*keymanagementv1.ExtractKeyRegionResponse, error) {
	return &keymanagementv1.ExtractKeyRegionResponse{}, nil
}
