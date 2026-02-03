package sessionmanager

import (
	"context"

	"google.golang.org/grpc"

	oidcmappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"
)

// FakeSessionManagerClient is a fake implementation of oidcmappinggrpc.OIDCMappingClient
// that can be used for testing purposes.
// sonarignore
type FakeSessionManagerClient struct {
	MockApplyOIDCMapping func(
		ctx context.Context,
		req *oidcmappinggrpc.ApplyOIDCMappingRequest,
	) (*oidcmappinggrpc.ApplyOIDCMappingResponse, error)

	MockBlockOIDCMapping func(
		ctx context.Context,
		req *oidcmappinggrpc.BlockOIDCMappingRequest,
	) (*oidcmappinggrpc.BlockOIDCMappingResponse, error)

	MockUnblockOIDCMapping func(
		ctx context.Context,
		req *oidcmappinggrpc.UnblockOIDCMappingRequest,
	) (*oidcmappinggrpc.UnblockOIDCMappingResponse, error)

	MockRemoveOIDCMapping func(
		ctx context.Context,
		req *oidcmappinggrpc.RemoveOIDCMappingRequest,
	) (*oidcmappinggrpc.RemoveOIDCMappingResponse, error)
}

// NewFakeSessionManagerClient creates a new fake session manager client
func NewFakeSessionManagerClient() *FakeSessionManagerClient {
	return &FakeSessionManagerClient{}
}

// ApplyOIDCMapping implements the oidcmappinggrpc.OIDCMappingClient interface
func (f *FakeSessionManagerClient) ApplyOIDCMapping(
	ctx context.Context,
	req *oidcmappinggrpc.ApplyOIDCMappingRequest,
	_ ...grpc.CallOption,
) (*oidcmappinggrpc.ApplyOIDCMappingResponse, error) {
	return f.MockApplyOIDCMapping(ctx, req)
}

func (f *FakeSessionManagerClient) BlockOIDCMapping(
	ctx context.Context,
	req *oidcmappinggrpc.BlockOIDCMappingRequest,
	_ ...grpc.CallOption,
) (*oidcmappinggrpc.BlockOIDCMappingResponse, error) {
	return f.MockBlockOIDCMapping(ctx, req)
}

func (f *FakeSessionManagerClient) UnblockOIDCMapping(
	ctx context.Context,
	req *oidcmappinggrpc.UnblockOIDCMappingRequest,
	_ ...grpc.CallOption,
) (*oidcmappinggrpc.UnblockOIDCMappingResponse, error) {
	return f.MockUnblockOIDCMapping(ctx, req)
}

// RemoveOIDCMapping implements the oidcmappinggrpc.OIDCMappingClient interface
func (f *FakeSessionManagerClient) RemoveOIDCMapping(
	ctx context.Context,
	req *oidcmappinggrpc.RemoveOIDCMappingRequest,
	_ ...grpc.CallOption,
) (*oidcmappinggrpc.RemoveOIDCMappingResponse, error) {
	return f.MockRemoveOIDCMapping(ctx, req)
}
