package sessionmanager

import (
	"context"

	"google.golang.org/grpc"

	trustmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/trustmapping/v1"
)

// FakeSessionManagerClient is a fake implementation of trustmappingv1.ServiceClient
// that can be used for testing purposes.
// sonarignore
type FakeSessionManagerClient struct {
	MockApplyTrustMapping func(
		ctx context.Context,
		req *trustmappingv1.ApplyTrustMappingRequest,
	) (*trustmappingv1.ApplyTrustMappingResponse, error)

	MockBlockTrustMapping func(
		ctx context.Context,
		req *trustmappingv1.BlockTrustMappingRequest,
	) (*trustmappingv1.BlockTrustMappingResponse, error)

	MockUnblockTrustMapping func(
		ctx context.Context,
		req *trustmappingv1.UnblockTrustMappingRequest,
	) (*trustmappingv1.UnblockTrustMappingResponse, error)

	MockRemoveTrustMapping func(
		ctx context.Context,
		req *trustmappingv1.RemoveTrustMappingRequest,
	) (*trustmappingv1.RemoveTrustMappingResponse, error)
}

// NewFakeSessionManagerClient creates a new fake session manager client
func NewFakeSessionManagerClient() *FakeSessionManagerClient {
	return &FakeSessionManagerClient{}
}

// ApplyTrustMapping implements the trustmappingv1.ServiceClient interface
func (f *FakeSessionManagerClient) ApplyTrustMapping(
	ctx context.Context,
	req *trustmappingv1.ApplyTrustMappingRequest,
	_ ...grpc.CallOption,
) (*trustmappingv1.ApplyTrustMappingResponse, error) {
	return f.MockApplyTrustMapping(ctx, req)
}

func (f *FakeSessionManagerClient) BlockTrustMapping(
	ctx context.Context,
	req *trustmappingv1.BlockTrustMappingRequest,
	_ ...grpc.CallOption,
) (*trustmappingv1.BlockTrustMappingResponse, error) {
	return f.MockBlockTrustMapping(ctx, req)
}

func (f *FakeSessionManagerClient) UnblockTrustMapping(
	ctx context.Context,
	req *trustmappingv1.UnblockTrustMappingRequest,
	_ ...grpc.CallOption,
) (*trustmappingv1.UnblockTrustMappingResponse, error) {
	return f.MockUnblockTrustMapping(ctx, req)
}

// RemoveTrustMapping implements the trustmappingv1.ServiceClient interface
func (f *FakeSessionManagerClient) RemoveTrustMapping(
	ctx context.Context,
	req *trustmappingv1.RemoveTrustMappingRequest,
	_ ...grpc.CallOption,
) (*trustmappingv1.RemoveTrustMappingResponse, error) {
	return f.MockRemoveTrustMapping(ctx, req)
}
