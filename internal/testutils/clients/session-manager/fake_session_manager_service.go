package sessionmanager

import (
	"context"

	trustmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/trustmapping/v1"
)

// FakeSessionManagerService is a fake implementation of TrustMapping ServiceServer
// that can be used for testing purposes, particularly for integration tests
// that need a gRPC server.
// sonarignore
type FakeSessionManagerService struct {
	trustmappingv1.UnimplementedServiceServer

	// Configurable behavior for ApplyTrustMapping
	ApplyTrustMappingError   error
	ApplyTrustMappingSuccess bool
	ApplyTrustMappingMessage string

	// Configurable behavior for BlockTrustMapping
	MockBlockTrustMapping func(
		ctx context.Context,
		req *trustmappingv1.BlockTrustMappingRequest,
	) (*trustmappingv1.BlockTrustMappingResponse, error)

	// Configurable behavior for UnblockTrustMapping
	MockUnblockTrustMapping func(
		ctx context.Context,
		req *trustmappingv1.UnblockTrustMappingRequest,
	) (*trustmappingv1.UnblockTrustMappingResponse, error)

	// Configurable behavior for RemoveTrustMapping
	RemoveTrustMappingError   error
	RemoveTrustMappingSuccess bool
	RemoveTrustMappingMessage string
}

// NewFakeSessionManagerService creates a new fake session manager service with default success behavior
func NewFakeSessionManagerService() *FakeSessionManagerService {
	return &FakeSessionManagerService{
		ApplyTrustMappingSuccess:  true,
		RemoveTrustMappingSuccess: true,
	}
}

// ApplyTrustMapping implements the trustmappingv1.ServiceServer interface
func (f *FakeSessionManagerService) ApplyTrustMapping(
	_ context.Context,
	_ *trustmappingv1.ApplyTrustMappingRequest,
) (*trustmappingv1.ApplyTrustMappingResponse, error) {
	if f.ApplyTrustMappingError != nil {
		return nil, f.ApplyTrustMappingError
	}

	builder := trustmappingv1.ApplyTrustMappingResponse_builder{
		Success: f.ApplyTrustMappingSuccess,
	}

	if f.ApplyTrustMappingMessage != "" {
		builder.Message = &f.ApplyTrustMappingMessage
	}

	return builder.Build(), nil
}

// BlockTrustMapping implements the trustmappingv1.ServiceServer interface
func (f *FakeSessionManagerService) BlockTrustMapping(
	ctx context.Context,
	req *trustmappingv1.BlockTrustMappingRequest,
) (*trustmappingv1.BlockTrustMappingResponse, error) {
	if f.MockBlockTrustMapping != nil {
		return f.MockBlockTrustMapping(ctx, req)
	}

	return &trustmappingv1.BlockTrustMappingResponse{}, nil
}

// UnblockTrustMapping implements the trustmappingv1.ServiceServer interface
func (f *FakeSessionManagerService) UnblockTrustMapping(
	ctx context.Context,
	req *trustmappingv1.UnblockTrustMappingRequest,
) (*trustmappingv1.UnblockTrustMappingResponse, error) {
	if f.MockUnblockTrustMapping != nil {
		return f.MockUnblockTrustMapping(ctx, req)
	}

	return &trustmappingv1.UnblockTrustMappingResponse{}, nil
}

// RemoveTrustMapping implements the trustmappingv1.ServiceServer interface
func (f *FakeSessionManagerService) RemoveTrustMapping(
	_ context.Context,
	_ *trustmappingv1.RemoveTrustMappingRequest,
) (*trustmappingv1.RemoveTrustMappingResponse, error) {
	if f.RemoveTrustMappingError != nil {
		return nil, f.RemoveTrustMappingError
	}

	builder := trustmappingv1.RemoveTrustMappingResponse_builder{
		Success: f.RemoveTrustMappingSuccess,
	}

	if f.RemoveTrustMappingMessage != "" {
		builder.Message = &f.RemoveTrustMappingMessage
	}

	return builder.Build(), nil
}
