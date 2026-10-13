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

	"github.com/openkcm/cmk/internal/config"
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

	resp := &keystoremanagement.CreateKeystoreResponse{}
	if mc := grpcResp.GetRoleManagementConfig(); mc != nil {
		resp.RoleManagementConfig = keystoremanagement.ManagementConfig{
			LocalityID: mc.GetLocalityId(),
			CommonName: mc.GetCommonName(),
		}
		if mc.GetAccessData() != nil {
			resp.RoleManagementConfig.AccessData = common.KeystoreConfig{
				Values: mc.GetAccessData().GetValues().AsMap(),
			}
		}
	}
	if mc := grpcResp.GetKeyManagementConfig(); mc != nil {
		resp.KeyManagementConfig = keystoremanagement.ManagementConfig{
			LocalityID: mc.GetLocalityId(),
			CommonName: mc.GetCommonName(),
		}
		if mc.GetAccessData() != nil {
			resp.KeyManagementConfig.AccessData = common.KeystoreConfig{
				Values: mc.GetAccessData().GetValues().AsMap(),
			}
		}
	}
	for _, r := range grpcResp.GetSupportedRegions() {
		resp.SupportedRegions = append(resp.SupportedRegions, config.Region{
			Name:          r.GetName(),
			TechnicalName: r.GetTechnicalName(),
		})
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

func (v1 *V1) GrantTrust(
	ctx context.Context,
	req *keystoremanagement.GrantTrustRequest,
) (*keystoremanagement.GrantTrustResponse, error) {
	value, err := structpb.NewStruct(req.Config.Values)
	if err != nil {
		return nil, fmt.Errorf(errFailedVParseProtoStructMsg, err)
	}
	var trustType grpckeystoremanagementv1.TrustType
	switch req.Type {
	case keystoremanagement.TrustTypeManagement:
		trustType = grpckeystoremanagementv1.TrustType_TRUST_TYPE_MANAGEMENT
	case keystoremanagement.TrustTypeCrypto:
		trustType = grpckeystoremanagementv1.TrustType_TRUST_TYPE_CRYPTO
	}

	in := &grpckeystoremanagementv1.GrantTrustRequest{
		Config:  &grpccommonv1.KeystoreInstanceConfig{Values: value},
		Subject: req.Subject,
		Region:  req.Region,
		Type:    trustType,
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf(errFailedValidationMsg, err)
	}

	grpcResp, err := v1.KeystoreProviderPluginClient.GrantTrust(ctx, in)
	if err != nil {
		return nil, err
	}

	resp := &keystoremanagement.GrantTrustResponse{}
	if grpcResp.GetAccessData() != nil {
		resp.AccessData = common.KeystoreConfig{Values: grpcResp.GetAccessData().AsMap()}
	}
	return resp, nil
}

func (v1 *V1) RemoveTrust(
	ctx context.Context,
	req *keystoremanagement.RemoveTrustRequest,
) (*keystoremanagement.RemoveTrustResponse, error) {
	cfgValue, err := structpb.NewStruct(req.Config.Values)
	if err != nil {
		return nil, fmt.Errorf(errFailedVParseProtoStructMsg, err)
	}
	accessDataValue, err := structpb.NewStruct(req.AccessData.Values)
	if err != nil {
		return nil, fmt.Errorf(errFailedVParseProtoStructMsg, err)
	}
	in := &grpckeystoremanagementv1.RemoveTrustRequest{
		Config:     &grpccommonv1.KeystoreInstanceConfig{Values: cfgValue},
		AccessData: accessDataValue,
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf(errFailedValidationMsg, err)
	}

	_, err = v1.KeystoreProviderPluginClient.RemoveTrust(ctx, in)
	if err != nil {
		return nil, err
	}
	return &keystoremanagement.RemoveTrustResponse{}, nil
}
