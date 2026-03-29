package mapping_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"

	mappingapi "github.com/openkcm/cmk/internal/apiregistry/api/mapping"
	apierrors "github.com/openkcm/cmk/internal/apiregistry/errors"
	"github.com/openkcm/cmk/internal/apiregistry/wrapper/mapping"
)

// mockMappingClient is a mock implementation of mappinggrpc.ServiceClient
type mockMappingClient struct {
	mapSystemToTenantFunc     func(ctx context.Context, req *mappinggrpc.MapSystemToTenantRequest) (*mappinggrpc.MapSystemToTenantResponse, error)
	unmapSystemFromTenantFunc func(ctx context.Context, req *mappinggrpc.UnmapSystemFromTenantRequest) (*mappinggrpc.UnmapSystemFromTenantResponse, error)
	getFunc                   func(ctx context.Context, req *mappinggrpc.GetRequest) (*mappinggrpc.GetResponse, error)
}

func (m *mockMappingClient) MapSystemToTenant(ctx context.Context, req *mappinggrpc.MapSystemToTenantRequest, opts ...grpc.CallOption) (*mappinggrpc.MapSystemToTenantResponse, error) {
	return m.mapSystemToTenantFunc(ctx, req)
}

func (m *mockMappingClient) UnmapSystemFromTenant(ctx context.Context, req *mappinggrpc.UnmapSystemFromTenantRequest, opts ...grpc.CallOption) (*mappinggrpc.UnmapSystemFromTenantResponse, error) {
	return m.unmapSystemFromTenantFunc(ctx, req)
}

func (m *mockMappingClient) Get(ctx context.Context, req *mappinggrpc.GetRequest, opts ...grpc.CallOption) (*mappinggrpc.GetResponse, error) {
	return m.getFunc(ctx, req)
}

func TestNewV1(t *testing.T) {
	mockClient := &mockMappingClient{}
	v1 := mapping.NewV1(mockClient)

	if v1 == nil {
		t.Fatal("expected non-nil V1 instance")
	}
}

func TestV1_MapSystemToTenant(t *testing.T) {
	tests := []struct {
		name          string
		request       *mappingapi.MapSystemToTenantRequest
		mockResponse  *mappinggrpc.MapSystemToTenantResponse
		mockError     error
		expectedError error
		expectSuccess bool
	}{
		{
			name: "successful mapping",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockResponse: &mappinggrpc.MapSystemToTenantResponse{
				Success: true,
			},
			expectSuccess: true,
		},
		{
			name: "missing external ID",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			expectedError: apierrors.NewValidationError("ExternalID", "external ID is required"),
		},
		{
			name: "missing type",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "",
				TenantID:   "tenant-456",
			},
			expectedError: apierrors.NewValidationError("Type", "type is required"),
		},
		{
			name: "missing tenant ID",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "",
			},
			expectedError: apierrors.NewValidationError("TenantID", "tenant ID is required"),
		},
		{
			name: "mapping already exists",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.AlreadyExists, "mapping already exists"),
			expectedError: apierrors.ErrMappingAlreadyExists,
		},
		{
			name: "mapping not found",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.NotFound, "mapping not found"),
			expectedError: apierrors.ErrMappingNotFound,
		},
		{
			name: "invalid external ID from server",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.InvalidArgument, "external ID is invalid"),
			expectedError: apierrors.ErrMappingInvalidExternalID,
		},
		{
			name: "invalid type from server",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.InvalidArgument, "type is invalid"),
			expectedError: apierrors.ErrInvalidType,
		},
		{
			name: "invalid tenant ID from server",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.InvalidArgument, "tenant ID is invalid"),
			expectedError: apierrors.ErrMappingInvalidTenantID,
		},
		{
			name: "system not mapped",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.FailedPrecondition, "system not mapped"),
			expectedError: apierrors.ErrSystemNotMapped,
		},
		{
			name: "generic operation failure",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.Internal, "internal error"),
			expectedError: apierrors.ErrMappingOperationFailed,
		},
		{
			name: "non-grpc error",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.Internal, "network error"),
			expectedError: apierrors.ErrMappingOperationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockMappingClient{
				mapSystemToTenantFunc: func(ctx context.Context, req *mappinggrpc.MapSystemToTenantRequest) (*mappinggrpc.MapSystemToTenantResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := mapping.NewV1(mockClient)
			resp, err := v1.MapSystemToTenant(context.Background(), tt.request)

			if tt.expectedError != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.expectedError)
				}
				if err.Error() != tt.expectedError.Error() {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Success != tt.expectSuccess {
				t.Errorf("expected success=%v, got %v", tt.expectSuccess, resp.Success)
			}
		})
	}
}

func TestV1_UnmapSystemFromTenant(t *testing.T) {
	tests := []struct {
		name          string
		request       *mappingapi.UnmapSystemFromTenantRequest
		mockResponse  *mappinggrpc.UnmapSystemFromTenantResponse
		mockError     error
		expectedError error
		expectSuccess bool
	}{
		{
			name: "successful unmapping",
			request: &mappingapi.UnmapSystemFromTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockResponse: &mappinggrpc.UnmapSystemFromTenantResponse{
				Success: true,
			},
			expectSuccess: true,
		},
		{
			name: "missing external ID",
			request: &mappingapi.UnmapSystemFromTenantRequest{
				ExternalID: "",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			expectedError: apierrors.NewValidationError("ExternalID", "external ID is required"),
		},
		{
			name: "missing type",
			request: &mappingapi.UnmapSystemFromTenantRequest{
				ExternalID: "ext-123",
				Type:       "",
				TenantID:   "tenant-456",
			},
			expectedError: apierrors.NewValidationError("Type", "type is required"),
		},
		{
			name: "missing tenant ID",
			request: &mappingapi.UnmapSystemFromTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "",
			},
			expectedError: apierrors.NewValidationError("TenantID", "tenant ID is required"),
		},
		{
			name: "mapping not found",
			request: &mappingapi.UnmapSystemFromTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.NotFound, "mapping not found"),
			expectedError: apierrors.ErrMappingNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockMappingClient{
				unmapSystemFromTenantFunc: func(ctx context.Context, req *mappinggrpc.UnmapSystemFromTenantRequest) (*mappinggrpc.UnmapSystemFromTenantResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := mapping.NewV1(mockClient)
			resp, err := v1.UnmapSystemFromTenant(context.Background(), tt.request)

			if tt.expectedError != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.expectedError)
				}
				if err.Error() != tt.expectedError.Error() {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Success != tt.expectSuccess {
				t.Errorf("expected success=%v, got %v", tt.expectSuccess, resp.Success)
			}
		})
	}
}

func TestV1_Get(t *testing.T) {
	tests := []struct {
		name             string
		request          *mappingapi.GetRequest
		mockResponse     *mappinggrpc.GetResponse
		mockError        error
		expectedError    error
		expectedTenantID string
	}{
		{
			name: "successful get",
			request: &mappingapi.GetRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
			},
			mockResponse: &mappinggrpc.GetResponse{
				TenantId: "tenant-456",
			},
			expectedTenantID: "tenant-456",
		},
		{
			name: "missing external ID",
			request: &mappingapi.GetRequest{
				ExternalID: "",
				Type:       "keystore",
			},
			expectedError: apierrors.NewValidationError("ExternalID", "external ID is required"),
		},
		{
			name: "missing type",
			request: &mappingapi.GetRequest{
				ExternalID: "ext-123",
				Type:       "",
			},
			expectedError: apierrors.NewValidationError("Type", "type is required"),
		},
		{
			name: "mapping not found",
			request: &mappingapi.GetRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
			},
			mockError:     status.Error(codes.NotFound, "mapping not found"),
			expectedError: apierrors.ErrMappingNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockMappingClient{
				getFunc: func(ctx context.Context, req *mappinggrpc.GetRequest) (*mappinggrpc.GetResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := mapping.NewV1(mockClient)
			resp, err := v1.Get(context.Background(), tt.request)

			if tt.expectedError != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.expectedError)
				}
				if err.Error() != tt.expectedError.Error() {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.TenantID != tt.expectedTenantID {
				t.Errorf("expected tenant ID %s, got %s", tt.expectedTenantID, resp.TenantID)
			}
		})
	}
}
