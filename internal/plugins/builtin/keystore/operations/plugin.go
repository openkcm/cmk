package operations

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/openkcm/plugin-sdk/pkg/hclog2slog"
	operationsv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
	slogctx "github.com/veqryn/slog-context"
)

const (
	pluginName = "keystore-operations-empty"
)

func V1BuiltIn() catalog.BuiltIn {
	return builtin(&V1Plugin{})
}

func builtin(p *V1Plugin) catalog.BuiltIn {
	return catalog.MakeBuiltIn(pluginName,
		operationsv1.KeystoreInstanceKeyOperationPluginServer(p),
		configv1.ConfigServiceServer(p))
}

type V1Plugin struct {
	configv1.UnsafeConfigServer
	operationsv1.KeystoreInstanceKeyOperationServer
}

var (
	_ operationsv1.KeystoreInstanceKeyOperationServer = (*V1Plugin)(nil)
	_ configv1.ConfigServer                           = (*V1Plugin)(nil)
)

// SetLogger method is called whenever the plugin start and giving the logger of host application
func (p *V1Plugin) SetLogger(logger hclog.Logger) {
	slog.SetDefault(hclog2slog.New(logger))
}

// Configure configures the plugin with the given configuration
func (p *V1Plugin) Configure(ctx context.Context, _ *configv1.ConfigureRequest) (*configv1.ConfigureResponse, error) {
	slog.DebugContext(ctx, "Builtin System Information Service (SIS) plugin")

	return &configv1.ConfigureResponse{}, nil
}

func (p *V1Plugin) GetKey(context.Context, *operationsv1.GetKeyRequest) (*operationsv1.GetKeyResponse, error) {
	slogctx.Debug(context.Background(), "Builtin Key Store Operations Service (KSO) - GetKey called")

	return &operationsv1.GetKeyResponse{}, nil
}

// CreateKey generates a new key with the specified algorithm
func (p *V1Plugin) CreateKey(context.Context, *operationsv1.CreateKeyRequest) (*operationsv1.CreateKeyResponse, error) {
	slogctx.Debug(context.Background(), "Builtin Key Store Operations Service (KSO) - CreateKey called")

	return &operationsv1.CreateKeyResponse{}, nil
}

// DeleteKey removes a key, optionally with a deletion window
func (p *V1Plugin) DeleteKey(context.Context, *operationsv1.DeleteKeyRequest) (*operationsv1.DeleteKeyResponse, error) {
	slogctx.Debug(context.Background(), "Builtin Key Store Operations Service (KSO) - DeleteKey called")

	return &operationsv1.DeleteKeyResponse{}, nil
}

// EnableKey activates a previously disabled key
func (p *V1Plugin) EnableKey(context.Context, *operationsv1.EnableKeyRequest) (*operationsv1.EnableKeyResponse, error) {
	slogctx.Debug(context.Background(), "Builtin Key Store Operations Service (KSO) - EnableKey called")

	return &operationsv1.EnableKeyResponse{}, nil
}

// DisableKey deactivates a key while maintaining its existence
func (p *V1Plugin) DisableKey(context.Context, *operationsv1.DisableKeyRequest) (*operationsv1.DisableKeyResponse, error) {
	slogctx.Debug(context.Background(), "Builtin Key Store Operations Service (KSO) - DisableKey called")

	return &operationsv1.DisableKeyResponse{}, nil
}

// Gets the parameters needed for importing key material
func (p *V1Plugin) GetImportParameters(context.Context, *operationsv1.GetImportParametersRequest) (*operationsv1.GetImportParametersResponse, error) {
	slogctx.Debug(context.Background(), "Builtin Key Store Operations Service (KSO) - GetImportParameters called")

	return &operationsv1.GetImportParametersResponse{}, nil
}

// Imports key material into a KMS key
func (p *V1Plugin) ImportKeyMaterial(context.Context, *operationsv1.ImportKeyMaterialRequest) (*operationsv1.ImportKeyMaterialResponse, error) {
	slogctx.Debug(context.Background(), "Builtin Key Store Operations Service (KSO) - ImportKeyMaterial called")

	return &operationsv1.ImportKeyMaterialResponse{}, nil
}

// Validate the key attributes against the plugin's requirements
func (p *V1Plugin) ValidateKey(context.Context, *operationsv1.ValidateKeyRequest) (*operationsv1.ValidateKeyResponse, error) {
	slogctx.Debug(context.Background(), "Builtin Key Store Operations Service (KSO) - ValidateKey called")

	return &operationsv1.ValidateKeyResponse{}, nil
}

// ValidateKeyAccessData checks the access data for key management and crypto operations
func (p *V1Plugin) ValidateKeyAccessData(context.Context, *operationsv1.ValidateKeyAccessDataRequest) (*operationsv1.ValidateKeyAccessDataResponse, error) {
	slogctx.Debug(context.Background(), "Builtin Key Store Operations Service (KSO) - ValidateKeyAccessData called")

	return &operationsv1.ValidateKeyAccessDataResponse{}, nil
}

// TransformCryptoAccessData transforms the JSON-stored crypto access data into protobuf wire format for a given key
func (p *V1Plugin) TransformCryptoAccessData(context.Context, *operationsv1.TransformCryptoAccessDataRequest) (*operationsv1.TransformCryptoAccessDataResponse, error) {
	slogctx.Debug(context.Background(), "Builtin Key Store Operations Service (KSO) - TransformCryptoAccessData called")

	return &operationsv1.TransformCryptoAccessDataResponse{}, nil
}

// ExtractKeyRegion extracts the region from key attributes
func (p *V1Plugin) ExtractKeyRegion(context.Context, *operationsv1.ExtractKeyRegionRequest) (*operationsv1.ExtractKeyRegionResponse, error) {
	slogctx.Debug(context.Background(), "Builtin Key Store Operations Service (KSO) - ExtractKeyRegion called")

	return &operationsv1.ExtractKeyRegionResponse{}, nil
}
