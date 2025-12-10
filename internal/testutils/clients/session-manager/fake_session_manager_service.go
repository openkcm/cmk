package sessionmanager

import (
	"context"

	oidcmappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"
)

// FakeSessionManagerService is a fake implementation of OIDCMapping ServiceServer
// that can be used for testing purposes, particularly for integration tests
// that need a gRPC server.
// sonarignore
type FakeSessionManagerService struct {
	oidcmappinggrpc.UnimplementedServiceServer

	// Configurable behavior for ApplyOIDCMapping
	ApplyOIDCMappingError   error
	ApplyOIDCMappingSuccess bool
	ApplyOIDCMappingMessage string

	// Configurable behavior for BlockOIDCMapping
	MockBlockOIDCMapping func(
		ctx context.Context,
		req *oidcmappinggrpc.BlockOIDCMappingRequest,
	) (*oidcmappinggrpc.BlockOIDCMappingResponse, error)

	// Configurable behavior for UnblockOIDCMapping
	MockUnblockOIDCMapping func(
		ctx context.Context,
		req *oidcmappinggrpc.UnblockOIDCMappingRequest,
	) (*oidcmappinggrpc.UnblockOIDCMappingResponse, error)

	// Configurable behavior for RemoveOIDCMapping
	RemoveOIDCMappingError   error
	RemoveOIDCMappingSuccess bool
	RemoveOIDCMappingMessage string
}

// NewFakeSessionManagerService creates a new fake session manager service with default success behavior
func NewFakeSessionManagerService() *FakeSessionManagerService {
	return &FakeSessionManagerService{
		ApplyOIDCMappingSuccess:  true,
		RemoveOIDCMappingSuccess: true,
	}
}

// ApplyOIDCMapping implements the oidcmappinggrpc.ServiceServer interface
func (f *FakeSessionManagerService) ApplyOIDCMapping(
	_ context.Context,
	_ *oidcmappinggrpc.ApplyOIDCMappingRequest,
) (*oidcmappinggrpc.ApplyOIDCMappingResponse, error) {
	if f.ApplyOIDCMappingError != nil {
		return nil, f.ApplyOIDCMappingError
	}

	response := &oidcmappinggrpc.ApplyOIDCMappingResponse{
		Success: f.ApplyOIDCMappingSuccess,
	}

	if f.ApplyOIDCMappingMessage != "" {
		response.Message = &f.ApplyOIDCMappingMessage
	}

	return response, nil
}

// BlockOIDCMapping implements the oidcmappinggrpc.ServiceServer interface
func (f *FakeSessionManagerService) BlockOIDCMapping(
	ctx context.Context,
	req *oidcmappinggrpc.BlockOIDCMappingRequest,
) (*oidcmappinggrpc.BlockOIDCMappingResponse, error) {
	if f.MockBlockOIDCMapping != nil {
		return f.MockBlockOIDCMapping(ctx, req)
	}

	return &oidcmappinggrpc.BlockOIDCMappingResponse{}, nil
}

// UnblockOIDCMapping implements the oidcmappinggrpc.ServiceServer interface
func (f *FakeSessionManagerService) UnblockOIDCMapping(
	ctx context.Context,
	req *oidcmappinggrpc.UnblockOIDCMappingRequest,
) (*oidcmappinggrpc.UnblockOIDCMappingResponse, error) {
	if f.MockUnblockOIDCMapping != nil {
		return f.MockUnblockOIDCMapping(ctx, req)
	}

	return &oidcmappinggrpc.UnblockOIDCMappingResponse{}, nil
}

// RemoveOIDCMapping implements the oidcmappinggrpc.ServiceServer interface
func (f *FakeSessionManagerService) RemoveOIDCMapping(
	_ context.Context,
	_ *oidcmappinggrpc.RemoveOIDCMappingRequest,
) (*oidcmappinggrpc.RemoveOIDCMappingResponse, error) {
	if f.RemoveOIDCMappingError != nil {
		return nil, f.RemoveOIDCMappingError
	}

	response := &oidcmappinggrpc.RemoveOIDCMappingResponse{
		Success: f.RemoveOIDCMappingSuccess,
	}

	if f.RemoveOIDCMappingMessage != "" {
		response.Message = &f.RemoveOIDCMappingMessage
	}

	return response, nil
}
