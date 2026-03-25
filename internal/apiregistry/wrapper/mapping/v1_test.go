package mapping

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"

	"github.com/openkcm/cmk/internal/apiregistry/api/mapping"
)

// mockMappingClient is a mock implementation of mappinggrpc.ServiceClient
type mockMappingClient struct {
	mapSystemToTenantFunc    func(ctx context.Context, req *mappinggrpc.MapSystemToTenantRequest) (*mappinggrpc.MapSystemToTenantResponse, error)
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
		name           string
		request        *mapping.MapSystemToTenantRequest
		mockResponse   *mappinggrpc.MapSystemToTenantResponse
		mockError      error
		expectedError  error
		expectSuccess  bool
	}{
		{
			name: "successful mapping",
			request: &mapping.MapSystemToTenantRequest{
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
			request: &mapping.MapSystemToTenantRequest{
				ExternalID: "",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			expectedError: mapping.NewValidationError("ExternalID", "external ID is required"),
		},
		{
			name: "missing type",
			request: &mapping.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "",
				TenantID:   "tenant-456",
			},
			expectedError: mapping.NewValidationError("Type", "type is required"),
		},
		{
			name: "missing tenant ID",
			request: &mapping.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "",
			},
			expectedError: mapping.NewValidationError("TenantID", "tenant ID is required"),
		},
		{
			name: "mapping already exists",
			request: &mapping.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.AlreadyExists, "mapping already exists"),
			expectedError: mapping.ErrMappingAlreadyExists,
		},
		{
			name: "mapping not found",
			request: &mapping.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.NotFound, "mapping not found"),
			expectedError: mapping.ErrMappingNotFound,
		},
		{
			name: "invalid external ID from server",
			request: &mapping.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.InvalidArgument, "external ID is invalid"),
			expectedError: mapping.ErrInvalidExternalID,
		},
		{
			name: "invalid type from server",
			request: &mapping.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.InvalidArgument, "type is invalid"),
			expectedError: mapping.ErrInvalidType,
		},
		{
			name: "invalid tenant ID from server",
			request: &mapping.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.InvalidArgument, "tenant ID is invalid"),
			expectedError: mapping.ErrInvalidTenantID,
		},
		{
			name: "system not mapped",
			request: &mapping.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.FailedPrecondition, "system not mapped"),
			expectedError: mapping.ErrSystemNotMapped,
		},
		{
			name: "generic operation failure",
			request: &mapping.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.Internal, "internal error"),
			expectedError: mapping.ErrOperationFailed,
		},
		{
			name: "non-grpc error",
			request: &mapping.MapSystemToTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     errors.New("network error"),
			expectedError: mapping.ErrOperationFailed,
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
		name           string
		request        *mapping.UnmapSystemFromTenantRequest
		mockResponse   *mappinggrpc.UnmapSystemFromTenantResponse
		mockError      error
		expectedError  error
		expectSuccess  bool
	}{
		{
			name: "successful unmapping",
			request: &mapping.UnmapSystemFromTenantRequest{
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
			request: &mapping.UnmapSystemFromTenantRequest{
				ExternalID: "",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			expectedError: mapping.NewValidationError("ExternalID", "external ID is required"),
		},
		{
			name: "missing type",
			request: &mapping.UnmapSystemFromTenantRequest{
				ExternalID: "ext-123",
				Type:       "",
				TenantID:   "tenant-456",
			},
			expectedError: mapping.NewValidationError("Type", "type is required"),
		},
		{
			name: "missing tenant ID",
			request: &mapping.UnmapSystemFromTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "",
			},
			expectedError: mapping.NewValidationError("TenantID", "tenant ID is required"),
		},
		{
			name: "mapping not found",
			request: &mapping.UnmapSystemFromTenantRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.NotFound, "mapping not found"),
			expectedError: mapping.ErrMappingNotFound,
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
		name           string
		request        *mapping.GetRequest
		mockResponse   *mappinggrpc.GetResponse
		mockError      error
		expectedError  error
		expectedTenantID string
	}{
		{
			name: "successful get",
			request: &mapping.GetRequest{
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
			request: &mapping.GetRequest{
				ExternalID: "",
				Type:       "keystore",
			},
			expectedError: mapping.NewValidationError("ExternalID", "external ID is required"),
		},
		{
			name: "missing type",
			request: &mapping.GetRequest{
				ExternalID: "ext-123",
				Type:       "",
			},
			expectedError: mapping.NewValidationError("Type", "type is required"),
		},
		{
			name: "mapping not found",
			request: &mapping.GetRequest{
				ExternalID: "ext-123",
				Type:       "keystore",
			},
			mockError:     status.Error(codes.NotFound, "mapping not found"),
			expectedError: mapping.ErrMappingNotFound,
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
			expectedError: mapping.ErrMappingNotFound,
		},
		{
			name:          "already exists",
			inputError:    status.Error(codes.AlreadyExists, "already exists"),
			expectedError: mapping.ErrMappingAlreadyExists,
		},
		{
			name:          "invalid argument with external",
			inputError:    status.Error(codes.InvalidArgument, "invalid external ID"),
			expectedError: mapping.ErrInvalidExternalID,
		},
		{
			name:          "invalid argument with type",
			inputError:    status.Error(codes.InvalidArgument, "invalid type"),
			expectedError: mapping.ErrInvalidType,
		},
		{
			name:          "invalid argument with tenant",
			inputError:    status.Error(codes.InvalidArgument, "invalid tenant ID"),
			expectedError: mapping.ErrInvalidTenantID,
		},
		{
			name:          "invalid argument without keyword",
			inputError:    status.Error(codes.InvalidArgument, "something wrong"),
			expectedError: mapping.ErrOperationFailed,
		},
		{
			name:          "failed precondition",
			inputError:    status.Error(codes.FailedPrecondition, "system not mapped"),
			expectedError: mapping.ErrSystemNotMapped,
		},
		{
			name:          "internal error",
			inputError:    status.Error(codes.Internal, "internal error"),
			expectedError: mapping.ErrOperationFailed,
		},
		{
			name:          "non-grpc error",
			inputError:    errors.New("network error"),
			expectedError: mapping.ErrOperationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertGRPCError(tt.inputError)
			if result != tt.expectedError {
				t.Errorf("convertGRPCError() = %v, want %v", result, tt.expectedError)
			}
		})
	}
}
