package mapping

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"

	mappingapi "github.com/openkcm/cmk/internal/apiregistry/api/mapping"
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
	v1 := NewV1(mockClient)

	if v1 == nil {
		t.Fatal("expected non-nil V1 instance")
	}
	if v1.client == nil {
		t.Error("expected client to be set")
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
			expectedError: mappingapi.NewValidationError("ExternalID", "external ID is required"),
		},
		{
			name: "missing type",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "",
				TenantID:   "tenant-456",
			},
			expectedError: mappingapi.NewValidationError("Type", "type is required"),
		},
		{
			name: "missing tenant ID",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "",
			},
			expectedError: mappingapi.NewValidationError("TenantID", "tenant ID is required"),
		},
		{
			name: "mapping already exists",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.AlreadyExists, "mapping already exists"),
			expectedError: mappingapi.ErrMappingAlreadyExists,
		},
		{
			name: "mapping not found",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.NotFound, "mapping not found"),
			expectedError: mappingapi.ErrMappingNotFound,
		},
		{
			name: "invalid external ID from server",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.InvalidArgument, "external ID is invalid"),
			expectedError: mappingapi.ErrInvalidExternalID,
		},
		{
			name: "invalid type from server",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.InvalidArgument, "type is invalid"),
			expectedError: mappingapi.ErrInvalidType,
		},
		{
			name: "invalid tenant ID from server",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.InvalidArgument, "tenant ID is invalid"),
			expectedError: mappingapi.ErrInvalidTenantID,
		},
		{
			name: "system not mapped",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.FailedPrecondition, "system not mapped"),
			expectedError: mappingapi.ErrSystemNotMapped,
		},
		{
			name: "generic operation failure",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.Internal, "internal error"),
			expectedError: mappingapi.ErrOperationFailed,
		},
		{
			name: "non-grpc error",
			request: &mappingapi.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     errors.New("network error"),
			expectedError: mappingapi.ErrOperationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockMappingClient{
				mapSystemToTenantFunc: func(ctx context.Context, req *mappinggrpc.MapSystemToTenantRequest) (*mappinggrpc.MapSystemToTenantResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
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
			expectedError: mappingapi.NewValidationError("ExternalID", "external ID is required"),
		},
		{
			name: "missing type",
			request: &mappingapi.UnmapSystemFromTenantRequest{
				ExternalID: "ext-123",
				Type:       "",
				TenantID:   "tenant-456",
			},
			expectedError: mappingapi.NewValidationError("Type", "type is required"),
		},
		{
			name: "missing tenant ID",
			request: &mappingapi.UnmapSystemFromTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "",
			},
			expectedError: mappingapi.NewValidationError("TenantID", "tenant ID is required"),
		},
		{
			name: "mapping not found",
			request: &mappingapi.UnmapSystemFromTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.NotFound, "mapping not found"),
			expectedError: mappingapi.ErrMappingNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockMappingClient{
				unmapSystemFromTenantFunc: func(ctx context.Context, req *mappinggrpc.UnmapSystemFromTenantRequest) (*mappinggrpc.UnmapSystemFromTenantResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
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
			expectedError: mappingapi.NewValidationError("ExternalID", "external ID is required"),
		},
		{
			name: "missing type",
			request: &mappingapi.GetRequest{
				ExternalID: "ext-123",
				Type:       "",
			},
			expectedError: mappingapi.NewValidationError("Type", "type is required"),
		},
		{
			name: "mapping not found",
			request: &mappingapi.GetRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
			},
			mockError:     status.Error(codes.NotFound, "mapping not found"),
			expectedError: mappingapi.ErrMappingNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockMappingClient{
				getFunc: func(ctx context.Context, req *mappinggrpc.GetRequest) (*mappinggrpc.GetResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
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

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "exact match",
			s:        "external",
			substr:   "external",
			expected: true,
		},
		{
			name:     "prefix match",
			s:        "external ID",
			substr:   "external",
			expected: true,
		},
		{
			name:     "suffix match",
			s:        "invalid external",
			substr:   "external",
			expected: true,
		},
		{
			name:     "middle match",
			s:        "the external is",
			substr:   "external",
			expected: true,
		},
		{
			name:     "no match",
			s:        "tenant ID",
			substr:   "external",
			expected: false,
		},
		{
			name:     "empty string",
			s:        "",
			substr:   "external",
			expected: false,
		},
		{
			name:     "empty substring",
			s:        "external",
			substr:   "",
			expected: true,
		},
		{
			name:     "substring longer than string",
			s:        "ext",
			substr:   "external",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestConvertGRPCError(t *testing.T) {
	tests := []struct {
		name          string
		inputError    error
		expectedError error
	}{
		{
			name:          "not found",
			inputError:    status.Error(codes.NotFound, "not found"),
			expectedError: mappingapi.ErrMappingNotFound,
		},
		{
			name:          "already exists",
			inputError:    status.Error(codes.AlreadyExists, "already exists"),
			expectedError: mappingapi.ErrMappingAlreadyExists,
		},
		{
			name:          "invalid argument with external",
			inputError:    status.Error(codes.InvalidArgument, "invalid external ID"),
			expectedError: mappingapi.ErrInvalidExternalID,
		},
		{
			name:          "invalid argument with type",
			inputError:    status.Error(codes.InvalidArgument, "invalid type"),
			expectedError: mappingapi.ErrInvalidType,
		},
		{
			name:          "invalid argument with tenant",
			inputError:    status.Error(codes.InvalidArgument, "invalid tenant ID"),
			expectedError: mappingapi.ErrInvalidTenantID,
		},
		{
			name:          "invalid argument without keyword",
			inputError:    status.Error(codes.InvalidArgument, "something wrong"),
			expectedError: mappingapi.ErrOperationFailed,
		},
		{
			name:          "failed precondition",
			inputError:    status.Error(codes.FailedPrecondition, "system not mapped"),
			expectedError: mappingapi.ErrSystemNotMapped,
		},
		{
			name:          "internal error",
			inputError:    status.Error(codes.Internal, "internal error"),
			expectedError: mappingapi.ErrOperationFailed,
		},
		{
			name:          "non-grpc error",
			inputError:    errors.New("network error"), //nolint:err113 // test error
			expectedError: mappingapi.ErrOperationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertGRPCError(tt.inputError)
			if result != tt.expectedError { //nolint:err113 // comparing error types in test
				t.Errorf("convertGRPCError() = %v, want %v", result, tt.expectedError)
			}
		})
	}
}
