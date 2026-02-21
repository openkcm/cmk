package keymanagement

import (
	"context"

	"github.com/openkcm/plugin-sdk/api"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/common"
)

type KeyManagement interface {
	ServiceInfo() api.Info

	GetKey(ctx context.Context, req *GetKeyRequest) (*GetKeyResponse, error)
	CreateKey(ctx context.Context, req *CreateKeyRequest) (*CreateKeyResponse, error)
	DeleteKey(ctx context.Context, req *DeleteKeyRequest) (*DeleteKeyResponse, error)
	EnableKey(ctx context.Context, req *EnableKeyRequest) (*EnableKeyResponse, error)
	GetImportParameters(ctx context.Context, req *GetImportParametersRequest) (*GetImportParametersResponse, error)
	ImportKeyMaterial(ctx context.Context, req *ImportKeyMaterialRequest) (*ImportKeyMaterialResponse, error)
	ValidateKey(ctx context.Context, req *ValidateKeyRequest) (*ValidateKeyResponse, error)
	ValidateKeyAccessData(
		ctx context.Context,
		req *ValidateKeyAccessDataRequest,
	) (*ValidateKeyAccessDataResponse, error)
	TransformCryptoAccessData(
		ctx context.Context,
		req *TransformCryptoAccessDataRequest,
	) (*TransformCryptoAccessDataResponse, error)
	ExtractKeyRegion(ctx context.Context, req *ExtractKeyRegionRequest) (*ExtractKeyRegionResponse, error)
}

type KeyAlgorithm int32

const (
	UnspecifiedKeyAlgorithm KeyAlgorithm = iota
	AES256K
	RSA3072
	RSA4096
)

type KeyType int32

const (
	UnspecifiedKeyType KeyType = iota
	SystemManaged
	BYOK
	HYOK
)

type RequestParameters struct {
	// V1 Fields
	Config common.KeystoreConfig
	KeyID  string
}

type GetKeyRequest struct {
	// V1 Fields
	Parameters RequestParameters
}

type GetKeyResponse struct {
	// V1 Fields
	KeyID        string
	KeyAlgorithm KeyAlgorithm
	Status       string
	Usage        string
}

// CreateKeyRequest contains parameters for key creation
type CreateKeyRequest struct {
	// V1 Fields
	Config       common.KeystoreConfig
	KeyAlgorithm KeyAlgorithm
	ID           *string
	Region       string
	KeyType      KeyType
}

type CreateKeyResponse struct {
	// V1 Fields
	KeyID  string
	Status string
}

// DeleteKeyRequest contains parameters for key deletion
type DeleteKeyRequest struct {
	// V1 Fields
	Parameters RequestParameters
	Window     *int32
}

type DeleteKeyResponse struct{}

// EnableKeyRequest contains parameters for key enablement
type EnableKeyRequest struct {
	// V1 Fields
	Parameters RequestParameters
}

type EnableKeyResponse struct{}

// DisableKeyRequest contains parameters for key disablement
type DisableKeyRequest struct {
	// V1 Fields
	Parameters RequestParameters
}

type DisableKeyResponse struct{}

type GetImportParametersRequest struct {
	// V1 Fields
	Parameters   RequestParameters
	KeyAlgorithm KeyAlgorithm
}

type GetImportParametersResponse struct {
	// V1 Fields
	KeyID            string
	ImportParameters map[string]any
}

type ImportKeyMaterialRequest struct {
	// V1 Fields
	Parameters           RequestParameters
	ImportParameters     map[string]any
	EncryptedKeyMaterial string
}

type ImportKeyMaterialResponse struct{}

type ValidateKeyRequest struct {
	// V1 Fields
	KeyType      KeyType
	KeyAlgorithm KeyAlgorithm
	Region       string
	NativeKeyID  string
}

type ValidateKeyResponse struct {
	// V1 Fields
	IsValid bool
	Message string
}

type ValidateKeyAccessDataRequest struct {
	// V1 Fields
	Management map[string]any
	Crypto     map[string]any
}

type ValidateKeyAccessDataResponse struct {
	// V1 Fields
	IsValid bool
	Message string
}

type TransformCryptoAccessDataRequest struct {
	// V1 Fields
	NativeKeyID string
	AccessData  []byte
}

type TransformCryptoAccessDataResponse struct {
	// V1 Fields
	TransformedAccessData map[string][]byte
}

type ExtractKeyRegionRequest struct {
	// V1 Fields
	NativeKeyID          string
	ManagementAccessData map[string]any
}

type ExtractKeyRegionResponse struct {
	// V1 Fields
	Region string
}
