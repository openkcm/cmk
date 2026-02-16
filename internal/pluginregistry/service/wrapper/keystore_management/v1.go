package keystore_management

import (
	"context"
	"fmt"

	"buf.build/go/protovalidate"
	"github.com/openkcm/plugin-sdk/api"
	"github.com/openkcm/plugin-sdk/pkg/plugin"
	"google.golang.org/protobuf/types/known/structpb"

	grpccommonv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/common/v1"
	grpckeystoremanagementv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/management/v1"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/common"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keystoremanagement"
)

const (
	errFailedValidationMsg        = "failed validation: %w"
	errFailedVParseProtoStructMsg = "failed to parse values into proto struct: %w"
)

type V1 struct {
	plugin.Facade
	grpckeystoremanagementv1.KeystoreProviderPluginClient
}

func (v1 *V1) Version() uint {
	return 1
}

func (v1 *V1) ServiceInfo() api.Info {
	return v1.Info
}

func (v1 *V1) CreateKeystore(
	ctx context.Context,
	req *keystoremanagement.CreateKeystoreRequest,
) (*keystoremanagement.CreateKeystoreResponse, error) {
	value, err := structpb.NewStruct(req.Values)
	if err != nil {
		return nil, fmt.Errorf(errFailedVParseProtoStructMsg, err)
	}

	in := &grpckeystoremanagementv1.CreateKeystoreRequest{
		Values: value,
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf(errFailedValidationMsg, err)
	}

	grpcResp, err := v1.KeystoreProviderPluginClient.CreateKeystore(ctx, in)
	if err != nil {
		return nil, err
	}
	resp := &keystoremanagement.CreateKeystoreResponse{
		Config: common.KeystoreConfig{
			Values: nil,
		},
	}
	if grpcResp.GetConfig() != nil || grpcResp.GetConfig().GetValues() != nil {
		resp.Config.Values = grpcResp.GetConfig().GetValues().AsMap()
	}
	return resp, nil
}

func (v1 *V1) DeleteKeystore(
	ctx context.Context,
	req *keystoremanagement.DeleteKeystoreRequest,
) (*keystoremanagement.DeleteKeystoreResponse, error) {
	value, err := structpb.NewStruct(req.Config.Values)
	if err != nil {
		return nil, fmt.Errorf(errFailedVParseProtoStructMsg, err)
	}
	in := &grpckeystoremanagementv1.DeleteKeystoreRequest{
		Config: &grpccommonv1.KeystoreInstanceConfig{
			Values: value,
		},
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf(errFailedValidationMsg, err)
	}

	_, err = v1.KeystoreProviderPluginClient.DeleteKeystore(ctx, in)
	if err != nil {
		return nil, err
	}
	return &keystoremanagement.DeleteKeystoreResponse{}, nil
}
