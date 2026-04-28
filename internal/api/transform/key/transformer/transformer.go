package transformer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	keystoreErrs "github.com/openkcm/plugin-sdk/pkg/plugin/keystore/errors"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/key/keyshared"
	"github.com/openkcm/cmk/internal/errs"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
)

const (
	GRPCErrorCodeInvalidAccessData errs.GRPCErrorCode = "INVALID_ACCESS_DATA"
)

var (
	ErrValidateKey            = errors.New("failed to validate key")
	ErrSerializeKeyAccessData = errors.New("failed to serialize key access data")
	ErrExtractKeyRegion       = errors.New("failed to extract key region from provider")
	ErrGRPCInvalidAccessData  = errs.GRPCError{
		Code:        GRPCErrorCodeInvalidAccessData,
		BaseMessage: "failed to validate access data for the keystore provider",
	}
)

type SerializedKeyAccessData struct {
	Management []byte
	Crypto     []byte
}

type ProviderTransformer interface {
	// ValidateAPI validates the key received from API requests against the provider's requirements.
	ValidateAPI(ctx context.Context, k cmkapi.Key) error

	ValidateKeyAccessData(ctx context.Context, k *cmkapi.KeyAccessDetails) error

	// SerializeKeyAccessData serializes the key access details into a format suitable for the provider.
	SerializeKeyAccessData(ctx context.Context, k *cmkapi.KeyAccessDetails) (*SerializedKeyAccessData, error)

	// GetRegion retrieves the region information for the given provider.
	GetRegion(ctx context.Context, k cmkapi.Key) (string, error)
}

type PluginProviderTransformer struct {
	provider     string
	pluginClient keymanagement.KeyManagement
}

func NewPluginProviderTransformer(
	pluginCatalog serviceapi.Registry,
	provider string,
) (*PluginProviderTransformer, error) {
	keyManagements, err := pluginCatalog.KeyManagements()
	if err != nil {
		return nil, errs.Wrapf(keyshared.ErrInvalidKeyProvider, provider)
	}

	pluginClient, ok := keyManagements[provider]
	if !ok {
		return nil, errs.Wrapf(keyshared.ErrInvalidKeyProvider, provider)
	}

	return &PluginProviderTransformer{
		provider:     provider,
		pluginClient: pluginClient,
	}, nil
}

func (v PluginProviderTransformer) ValidateAPI(ctx context.Context, k cmkapi.Key) error {
	validationRequest := keymanagement.ValidateKeyRequest{
		KeyType:      convertKeyType(k.Type),
		KeyAlgorithm: convertKeyAlgorithm(*k.Algorithm),
		Region:       *k.Region,
	}
	if k.NativeID != nil {
		validationRequest.NativeKeyID = *k.NativeID
	}

	response, err := v.pluginClient.ValidateKey(ctx, &validationRequest)
	if err != nil {
		return errs.Wrap(ErrValidateKey, err)
	}

	if !response.IsValid {
		return errs.Wrapf(ErrValidateKey, response.Message)
	}

	return nil
}

func (v PluginProviderTransformer) ValidateKeyAccessData(
	ctx context.Context,
	accessDetails *cmkapi.KeyAccessDetails,
) error {
	var management map[string]any
	if accessDetails.Management != nil {
		management = *accessDetails.Management
	}

	var crypto map[string]map[string]any
	if accessDetails.Crypto != nil {
		crypto = *accessDetails.Crypto
	}

	response, err := v.pluginClient.ValidateKeyAccessData(ctx, &keymanagement.ValidateKeyAccessDataRequest{
		Management: management,
		Crypto:     crypto,
	})
	if err != nil {
		if keystoreErrs.IsStatus(err, keystoreErrs.StatusInvalidKeyAccessData) {
			detailedErr := ErrGRPCInvalidAccessData.FromStatusError(err)
			return errors.Join(ErrSerializeKeyAccessData, detailedErr)
		}

		return errs.Wrap(ErrSerializeKeyAccessData, err)
	}

	if !response.IsValid {
		return errs.Wrapf(ErrSerializeKeyAccessData, response.Message)
	}

	return nil
}

func (v PluginProviderTransformer) SerializeKeyAccessData(
	ctx context.Context,
	keyAccessDetails *cmkapi.KeyAccessDetails,
) (*SerializedKeyAccessData, error) {
	err := v.ValidateKeyAccessData(ctx, keyAccessDetails)
	if err != nil {
		return nil, err
	}

	managementAccessData, err := json.Marshal(keyAccessDetails.Management)
	if err != nil {
		return nil, errs.Wrap(ErrSerializeKeyAccessData, err)
	}

	cryptoAccessData, err := json.Marshal(keyAccessDetails.Crypto)
	if err != nil {
		return nil, errs.Wrap(ErrSerializeKeyAccessData, err)
	}

	return &SerializedKeyAccessData{
		Management: managementAccessData,
		Crypto:     cryptoAccessData,
	}, nil
}

func (v PluginProviderTransformer) GetRegion(
	ctx context.Context,
	key cmkapi.Key,
) (string, error) {
	var management map[string]any
	if key.AccessDetails != nil && key.AccessDetails.Management != nil {
		management = *key.AccessDetails.Management
	}

	response, err := v.pluginClient.ExtractKeyRegion(ctx, &keymanagement.ExtractKeyRegionRequest{
		NativeKeyID:          *key.NativeID,
		ManagementAccessData: management,
	})
	if err != nil {
		return "", errs.Wrapf(ErrExtractKeyRegion, fmt.Sprintf("failed to extract key region: %v", err))
	}

	if response.Region == "" {
		return "", errs.Wrapf(ErrExtractKeyRegion, "extracted region is empty")
	}

	return response.Region, nil
}

func convertKeyType(keyType cmkapi.KeyType) keymanagement.KeyType {
	switch keyType {
	case cmkapi.KeyTypeBYOK:
		return keymanagement.BYOK
	case cmkapi.KeyTypeHYOK:
		return keymanagement.HYOK
	}

	return keymanagement.UnspecifiedKeyType
}

func convertKeyAlgorithm(algorithm cmkapi.KeyAlgorithm) keymanagement.KeyAlgorithm {
	switch algorithm {
	case cmkapi.KeyAlgorithmAES256:
		return keymanagement.AES256
	case cmkapi.KeyAlgorithmRSA3072:
		return keymanagement.RSA3072
	case cmkapi.KeyAlgorithmRSA4096:
		return keymanagement.RSA4096
	}

	return keymanagement.UnspecifiedKeyAlgorithm
}
