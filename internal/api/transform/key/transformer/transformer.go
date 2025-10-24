package transformer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/openkcm/plugin-sdk/pkg/catalog"

	keystoreopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/api/transform/key/keyshared"
	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/utils/protobuf"
)

var (
	ErrValidateKey            = errors.New("failed to validate key")
	ErrSerializeKeyAccessData = errors.New("failed to serialize key access data")
	ErrExtractKeyRegion       = errors.New("failed to extract key region from provider")
)

type SerializedKeyAccessData struct {
	Management []byte
	Crypto     []byte
}

type ProviderTransformer interface {
	// ValidateAPI validates the key received from API requests against the provider's requirements.
	ValidateAPI(ctx context.Context, k cmkapi.Key) error

	// SerializeKeyAccessData serializes the key access details into a format suitable for the provider.
	SerializeKeyAccessData(ctx context.Context, k cmkapi.Key) (*SerializedKeyAccessData, error)

	// GetRegion retrieves the region information for the given provider.
	GetRegion(ctx context.Context, k cmkapi.Key) (string, error)
}

type PluginProviderTransformer struct {
	provider     string
	pluginClient keystoreopv1.KeystoreInstanceKeyOperationClient
}

func NewPluginProviderTransformer(pluginCatalog *catalog.Catalog, provider string) (*PluginProviderTransformer, error) {
	plugin := pluginCatalog.LookupByTypeAndName(keystoreopv1.Type, provider)
	if plugin == nil {
		return nil, errs.Wrapf(keyshared.ErrInvalidKeyProvider, provider)
	}

	pluginClient := keystoreopv1.NewKeystoreInstanceKeyOperationClient(plugin.ClientConnection())

	return &PluginProviderTransformer{
		provider:     provider,
		pluginClient: pluginClient,
	}, nil
}

func (v PluginProviderTransformer) ValidateAPI(ctx context.Context, k cmkapi.Key) error {
	validationRequest := keystoreopv1.ValidateKeyRequest{
		KeyType:   convertKeyType(k.Type),
		Algorithm: convertKeyAlgorithm(*k.Algorithm),
		Region:    *k.Region,
	}
	if k.NativeID != nil {
		validationRequest.NativeKeyId = *k.NativeID
	}

	response, err := v.pluginClient.ValidateKey(ctx, &validationRequest)
	if err != nil {
		return errs.Wrap(ErrValidateKey, err)
	}

	if !response.GetIsValid() {
		return errs.Wrapf(ErrValidateKey, response.GetMessage())
	}

	return nil
}

func (v PluginProviderTransformer) SerializeKeyAccessData(
	ctx context.Context,
	key cmkapi.Key,
) (*SerializedKeyAccessData, error) {
	management, err := protobuf.StructToProtobuf(key.AccessDetails.Management)
	if err != nil {
		return nil, errs.Wrap(ErrSerializeKeyAccessData, err)
	}

	crypto, err := protobuf.StructToProtobuf(key.AccessDetails.Crypto)
	if err != nil {
		return nil, errs.Wrap(ErrSerializeKeyAccessData, err)
	}

	response, err := v.pluginClient.ValidateKeyAccessData(ctx, &keystoreopv1.ValidateKeyAccessDataRequest{
		Management: management,
		Crypto:     crypto,
	})
	if err != nil {
		return nil, errs.Wrap(ErrSerializeKeyAccessData, err)
	}

	if !response.GetIsValid() {
		return nil, errs.Wrapf(ErrSerializeKeyAccessData, response.GetMessage())
	}

	managementAccessData, err := json.Marshal(key.AccessDetails.Management)
	if err != nil {
		return nil, errs.Wrap(ErrSerializeKeyAccessData, err)
	}

	cryptoAccessData, err := json.Marshal(key.AccessDetails.Crypto)
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
	management, err := protobuf.StructToProtobuf(key.AccessDetails.Management)
	if err != nil {
		return "", errs.Wrap(ErrSerializeKeyAccessData, err)
	}

	response, err := v.pluginClient.ExtractKeyRegion(ctx, &keystoreopv1.ExtractKeyRegionRequest{
		NativeKeyId:          *key.NativeID,
		ManagementAccessData: management,
	})
	if err != nil {
		return "", errs.Wrapf(ErrExtractKeyRegion, fmt.Sprintf("failed to extract key region: %v", err))
	}

	if response.GetRegion() == "" {
		return "", errs.Wrapf(ErrExtractKeyRegion, "extracted region is empty")
	}

	return response.GetRegion(), nil
}

func convertKeyType(keyType cmkapi.KeyType) keystoreopv1.KeyType {
	switch keyType {
	case cmkapi.KeyTypeSYSTEMMANAGED:
		return keystoreopv1.KeyType_KEY_TYPE_SYSTEM_MANAGED
	case cmkapi.KeyTypeBYOK:
		return keystoreopv1.KeyType_KEY_TYPE_BYOK
	case cmkapi.KeyTypeHYOK:
		return keystoreopv1.KeyType_KEY_TYPE_HYOK
	}

	return keystoreopv1.KeyType_KEY_TYPE_UNSPECIFIED
}

func convertKeyAlgorithm(algorithm cmkapi.KeyAlgorithm) keystoreopv1.KeyAlgorithm {
	switch algorithm {
	case cmkapi.KeyAlgorithmAES256:
		return keystoreopv1.KeyAlgorithm_KEY_ALGORITHM_AES256
	case cmkapi.KeyAlgorithmRSA3072:
		return keystoreopv1.KeyAlgorithm_KEY_ALGORITHM_RSA3072
	case cmkapi.KeyAlgorithmRSA4096:
		return keystoreopv1.KeyAlgorithm_KEY_ALGORITHM_RSA4096
	}

	return keystoreopv1.KeyAlgorithm_KEY_ALGORITHM_UNSPECIFIED
}
