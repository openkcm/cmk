package key_management

import (
	"context"
	"fmt"

	"buf.build/go/protovalidate"
	"github.com/openkcm/plugin-sdk/api"
	"github.com/openkcm/plugin-sdk/pkg/plugin"
	"google.golang.org/protobuf/types/known/structpb"

	grpccommonv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/common/v1"
	grpckeymanagerv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
)

const (
	errFailedValidationMsg        = "failed validation: %w"
	errFailedVParseProtoStructMsg = "failed to parse values into proto struct: %w"
)

type V1 struct {
	plugin.Facade
	grpckeymanagerv1.KeystoreInstanceKeyOperationPluginClient
}

func (v1 *V1) Version() uint {
	return 1
}

func (v1 *V1) ServiceInfo() api.Info {
	return v1.Info
}

func (v1 *V1) GetKey(ctx context.Context, req *keymanagement.GetKeyRequest) (*keymanagement.GetKeyResponse, error) {
	value, err := structpb.NewStruct(req.Parameters.Config.Values)
	if err != nil {
		return nil, fmt.Errorf(errFailedVParseProtoStructMsg, err)
	}

	in := &grpckeymanagerv1.GetKeyRequest{
		Parameters: &grpckeymanagerv1.RequestParameters{
			Config: &grpccommonv1.KeystoreInstanceConfig{
				Values: value,
			},
			KeyId: req.Parameters.KeyID,
		},
	}
	grpcResp, err := v1.KeystoreInstanceKeyOperationPluginClient.GetKey(ctx, in)
	if err != nil {
		return nil, err
	}

	return &keymanagement.GetKeyResponse{
		KeyID:        grpcResp.GetKeyId(),
		KeyAlgorithm: keymanagement.KeyAlgorithm(grpcResp.GetAlgorithm()),
		Status:       grpcResp.GetStatus(),
		Usage:        grpcResp.GetUsage(),
	}, nil
}

func (v1 *V1) CreateKey(
	ctx context.Context,
	req *keymanagement.CreateKeyRequest,
) (*keymanagement.CreateKeyResponse, error) {
	value, err := structpb.NewStruct(req.Config.Values)
	if err != nil {
		return nil, fmt.Errorf(errFailedVParseProtoStructMsg, err)
	}

	in := &grpckeymanagerv1.CreateKeyRequest{
		Config: &grpccommonv1.KeystoreInstanceConfig{
			Values: value,
		},
		Algorithm: grpckeymanagerv1.KeyAlgorithm(req.KeyAlgorithm),
		Id:        req.ID,
		Region:    req.Region,
		KeyType:   grpckeymanagerv1.KeyType(req.KeyType),
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf(errFailedValidationMsg, err)
	}

	grpcResp, err := v1.KeystoreInstanceKeyOperationPluginClient.CreateKey(ctx, in)
	if err != nil {
		return nil, err
	}

	return &keymanagement.CreateKeyResponse{
		KeyID:  grpcResp.GetKeyId(),
		Status: grpcResp.GetStatus(),
	}, nil
}

func (v1 *V1) DeleteKey(
	ctx context.Context,
	req *keymanagement.DeleteKeyRequest,
) (*keymanagement.DeleteKeyResponse, error) {
	value, err := structpb.NewStruct(req.Parameters.Config.Values)
	if err != nil {
		return nil, fmt.Errorf(errFailedVParseProtoStructMsg, err)
	}

	in := &grpckeymanagerv1.DeleteKeyRequest{
		Parameters: &grpckeymanagerv1.RequestParameters{
			Config: &grpccommonv1.KeystoreInstanceConfig{
				Values: value,
			},
			KeyId: req.Parameters.KeyID,
		},
		Window: req.Window,
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf(errFailedValidationMsg, err)
	}

	_, err = v1.KeystoreInstanceKeyOperationPluginClient.DeleteKey(ctx, in)
	if err != nil {
		return nil, err
	}

	return &keymanagement.DeleteKeyResponse{}, nil
}

func (v1 *V1) EnableKey(
	ctx context.Context,
	req *keymanagement.EnableKeyRequest,
) (*keymanagement.EnableKeyResponse, error) {
	value, err := structpb.NewStruct(req.Parameters.Config.Values)
	if err != nil {
		return nil, fmt.Errorf(errFailedVParseProtoStructMsg, err)
	}

	in := &grpckeymanagerv1.EnableKeyRequest{
		Parameters: &grpckeymanagerv1.RequestParameters{
			Config: &grpccommonv1.KeystoreInstanceConfig{
				Values: value,
			},
			KeyId: req.Parameters.KeyID,
		},
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf(errFailedValidationMsg, err)
	}

	_, err = v1.KeystoreInstanceKeyOperationPluginClient.EnableKey(ctx, in)
	if err != nil {
		return nil, err
	}

	return &keymanagement.EnableKeyResponse{}, nil
}

func (v1 *V1) GetImportParameters(
	ctx context.Context,
	req *keymanagement.GetImportParametersRequest,
) (*keymanagement.GetImportParametersResponse, error) {
	value, err := structpb.NewStruct(req.Parameters.Config.Values)
	if err != nil {
		return nil, fmt.Errorf(errFailedVParseProtoStructMsg, err)
	}

	in := &grpckeymanagerv1.GetImportParametersRequest{
		Parameters: &grpckeymanagerv1.RequestParameters{
			Config: &grpccommonv1.KeystoreInstanceConfig{
				Values: value,
			},
			KeyId: req.Parameters.KeyID,
		},
		Algorithm: grpckeymanagerv1.KeyAlgorithm(req.KeyAlgorithm),
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf(errFailedValidationMsg, err)
	}

	grpcResp, err := v1.KeystoreInstanceKeyOperationPluginClient.GetImportParameters(ctx, in)
	if err != nil {
		return nil, err
	}

	return &keymanagement.GetImportParametersResponse{
		KeyID:            grpcResp.GetKeyId(),
		ImportParameters: grpcResp.GetImportParameters().AsMap(),
	}, nil
}

func (v1 *V1) ImportKeyMaterial(
	ctx context.Context,
	req *keymanagement.ImportKeyMaterialRequest,
) (*keymanagement.ImportKeyMaterialResponse, error) {
	value, err := structpb.NewStruct(req.Parameters.Config.Values)
	if err != nil {
		return nil, fmt.Errorf(errFailedVParseProtoStructMsg, err)
	}

	importParams, err := structpb.NewStruct(req.ImportParameters)
	if err != nil {
		return nil, fmt.Errorf(errFailedVParseProtoStructMsg, err)
	}

	in := &grpckeymanagerv1.ImportKeyMaterialRequest{
		Parameters: &grpckeymanagerv1.RequestParameters{
			Config: &grpccommonv1.KeystoreInstanceConfig{
				Values: value,
			},
			KeyId: req.Parameters.KeyID,
		},
		ImportParameters: importParams,
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf(errFailedValidationMsg, err)
	}

	_, err = v1.KeystoreInstanceKeyOperationPluginClient.ImportKeyMaterial(ctx, in)
	if err != nil {
		return nil, err
	}

	return &keymanagement.ImportKeyMaterialResponse{}, nil
}

func (v1 *V1) ValidateKey(
	ctx context.Context,
	req *keymanagement.ValidateKeyRequest,
) (*keymanagement.ValidateKeyResponse, error) {
	in := &grpckeymanagerv1.ValidateKeyRequest{
		KeyType:     grpckeymanagerv1.KeyType(req.KeyType),
		Algorithm:   grpckeymanagerv1.KeyAlgorithm(req.KeyAlgorithm),
		Region:      req.Region,
		NativeKeyId: req.NativeKeyID,
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf(errFailedValidationMsg, err)
	}

	grpcResp, err := v1.KeystoreInstanceKeyOperationPluginClient.ValidateKey(ctx, in)
	if err != nil {
		return nil, err
	}

	return &keymanagement.ValidateKeyResponse{
		IsValid: grpcResp.GetIsValid(),
		Message: grpcResp.GetMessage(),
	}, nil
}

func (v1 *V1) ValidateKeyAccessData(
	ctx context.Context,
	req *keymanagement.ValidateKeyAccessDataRequest,
) (*keymanagement.ValidateKeyAccessDataResponse, error) {
	management, err := structpb.NewStruct(req.Management)
	if err != nil {
		return nil, fmt.Errorf(errFailedVParseProtoStructMsg, err)
	}
	crypto, err := structpb.NewStruct(req.Crypto)
	if err != nil {
		return nil, fmt.Errorf(errFailedVParseProtoStructMsg, err)
	}

	in := &grpckeymanagerv1.ValidateKeyAccessDataRequest{
		Management: management,
		Crypto:     crypto,
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf(errFailedValidationMsg, err)
	}

	grpcResp, err := v1.KeystoreInstanceKeyOperationPluginClient.ValidateKeyAccessData(ctx, in)
	if err != nil {
		return nil, err
	}

	return &keymanagement.ValidateKeyAccessDataResponse{
		IsValid: grpcResp.GetIsValid(),
		Message: grpcResp.GetMessage(),
	}, nil
}

func (v1 *V1) TransformCryptoAccessData(
	ctx context.Context,
	req *keymanagement.TransformCryptoAccessDataRequest,
) (*keymanagement.TransformCryptoAccessDataResponse, error) {
	in := &grpckeymanagerv1.TransformCryptoAccessDataRequest{
		NativeKeyId: req.NativeKeyID,
		AccessData:  req.AccessData,
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf(errFailedValidationMsg, err)
	}

	grpcResp, err := v1.KeystoreInstanceKeyOperationPluginClient.TransformCryptoAccessData(ctx, in)
	if err != nil {
		return nil, err
	}

	return &keymanagement.TransformCryptoAccessDataResponse{
		TransformedAccessData: grpcResp.GetTransformedAccessData(),
	}, nil
}

func (v1 *V1) ExtractKeyRegion(ctx context.Context,
	req *keymanagement.ExtractKeyRegionRequest,
) (*keymanagement.ExtractKeyRegionResponse, error) {
	management, err := structpb.NewStruct(req.ManagementAccessData)
	if err != nil {
		return nil, fmt.Errorf(errFailedVParseProtoStructMsg, err)
	}

	in := &grpckeymanagerv1.ExtractKeyRegionRequest{
		NativeKeyId:          req.NativeKeyID,
		ManagementAccessData: management,
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf(errFailedValidationMsg, err)
	}

	grpcResp, err := v1.KeystoreInstanceKeyOperationPluginClient.ExtractKeyRegion(ctx, in)
	if err != nil {
		return nil, err
	}

	return &keymanagement.ExtractKeyRegionResponse{
		Region: grpcResp.GetRegion(),
	}, nil
}
