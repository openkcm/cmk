package system

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"

	"github.com/openkcm/cmk/internal/apiregistry/api/system"
)

// mockSystemClient is a mock implementation of systemgrpc.ServiceClient
type mockSystemClient struct {
	listSystemsFunc            func(ctx context.Context, req *systemgrpc.ListSystemsRequest) (*systemgrpc.ListSystemsResponse, error)
	registerSystemFunc         func(ctx context.Context, req *systemgrpc.RegisterSystemRequest) (*systemgrpc.RegisterSystemResponse, error)
	updateSystemL1KeyClaimFunc func(ctx context.Context, req *systemgrpc.UpdateSystemL1KeyClaimRequest) (*systemgrpc.UpdateSystemL1KeyClaimResponse, error)
	deleteSystemFunc           func(ctx context.Context, req *systemgrpc.DeleteSystemRequest) (*systemgrpc.DeleteSystemResponse, error)
	systemgrpc.ServiceClient // embed to satisfy interface
}

func (m *mockSystemClient) ListSystems(ctx context.Context, req *systemgrpc.ListSystemsRequest, opts ...grpc.CallOption) (*systemgrpc.ListSystemsResponse, error) {
	return m.listSystemsFunc(ctx, req)
}

func (m *mockSystemClient) RegisterSystem(ctx context.Context, req *systemgrpc.RegisterSystemRequest, opts ...grpc.CallOption) (*systemgrpc.RegisterSystemResponse, error) {
	return m.registerSystemFunc(ctx, req)
}

func (m *mockSystemClient) UpdateSystemL1KeyClaim(ctx context.Context, req *systemgrpc.UpdateSystemL1KeyClaimRequest, opts ...grpc.CallOption) (*systemgrpc.UpdateSystemL1KeyClaimResponse, error) {
	return m.updateSystemL1KeyClaimFunc(ctx, req)
}

func (m *mockSystemClient) DeleteSystem(ctx context.Context, req *systemgrpc.DeleteSystemRequest, opts ...grpc.CallOption) (*systemgrpc.DeleteSystemResponse, error) {
	return m.deleteSystemFunc(ctx, req)
}

func TestNewV1(t *testing.T) {
	mockClient := &mockSystemClient{}
	v1 := NewV1(mockClient)

	if v1 == nil {
		t.Fatal("expected non-nil V1 instance")
	}
	if v1.client == nil {
		t.Error("expected client to be set")
	}
}

func TestV1_ListSystems(t *testing.T) {
	tests := []struct {
		name           string
		request        *system.ListSystemsRequest
		mockResponse   *systemgrpc.ListSystemsResponse
		mockError      error
		expectedError  error
		expectedCount  int
	}{
		{
			name: "successful list",
			request: &system.ListSystemsRequest{
				Region:  "us-east-1",
				Limit:   10,
			},
			mockResponse: &systemgrpc.ListSystemsResponse{
				Systems: []*systemgrpc.System{
					{
						Region:        "us-east-1",
						ExternalId:    "ext-123",
						Type:          "KEYSTORE",
						TenantId:      "tenant-456",
						L2KeyId:       "l2-key",
						HasL1KeyClaim: true,
					},
				},
				NextPageToken: "next-token",
			},
			expectedCount: 1,
		},
		{
			name: "nil request",
			request: nil,
			expectedError: system.NewValidationError("request", "request cannot be nil"),
		},
		{
			name: "invalid limit",
			request: &system.ListSystemsRequest{
				Limit: -1,
			},
			expectedError: system.ErrInvalidLimit,
		},
		{
			name: "system not found",
			request: &system.ListSystemsRequest{
				Region:     "us-east-1",
				ExternalID: "non-existent",
			},
			mockError:     status.Error(codes.NotFound, "system not found"),
			expectedError: system.ErrSystemNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockSystemClient{
				listSystemsFunc: func(ctx context.Context, req *systemgrpc.ListSystemsRequest) (*systemgrpc.ListSystemsResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
			resp, err := v1.ListSystems(context.Background(), tt.request)

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

			if len(resp.Systems) != tt.expectedCount {
				t.Errorf("expected %d systems, got %d", tt.expectedCount, len(resp.Systems))
			}
		})
	}
}

func TestV1_RegisterSystem(t *testing.T) {
	tests := []struct {
		name           string
		request        *system.RegisterSystemRequest
		mockResponse   *systemgrpc.RegisterSystemResponse
		mockError      error
		expectedError  error
	}{
		{
			name: "successful registration",
			request: &system.RegisterSystemRequest{
				Region:        "us-east-1",
				ExternalID:    "ext-123",
				Type:          system.SystemTypeKeystore,
				TenantID:      "tenant-456",
				L2KeyID:       "l2-key",
				HasL1KeyClaim: true,
			},
			mockResponse: &systemgrpc.RegisterSystemResponse{},
		},
		{
			name:          "nil request",
			request:       nil,
			expectedError: system.NewValidationError("request", "request cannot be nil"),
		},
		{
			name: "missing region",
			request: &system.RegisterSystemRequest{
				ExternalID:    "ext-123",
				Type:          system.SystemTypeKeystore,
				TenantID:      "tenant-456",
			},
			expectedError: system.NewValidationError("Region", "region is required"),
		},
		{
			name: "missing external ID",
			request: &system.RegisterSystemRequest{
				Region:   "us-east-1",
				Type:     system.SystemTypeKeystore,
				TenantID: "tenant-456",
			},
			expectedError: system.NewValidationError("ExternalID", "external_id is required"),
		},
		{
			name: "missing tenant ID",
			request: &system.RegisterSystemRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				Type:       system.SystemTypeKeystore,
			},
			expectedError: system.NewValidationError("TenantID", "tenant_id is required"),
		},
		{
			name: "unspecified type",
			request: &system.RegisterSystemRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				Type:       system.SystemTypeUnspecified,
				TenantID:   "tenant-456",
			},
			expectedError: system.NewValidationError("Type", "type must be specified"),
		},
		{
			name: "system already exists",
			request: &system.RegisterSystemRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				Type:       system.SystemTypeKeystore,
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.AlreadyExists, "system already exists"),
			expectedError: system.ErrSystemAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockSystemClient{
				registerSystemFunc: func(ctx context.Context, req *systemgrpc.RegisterSystemRequest) (*systemgrpc.RegisterSystemResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
			_, err := v1.RegisterSystem(context.Background(), tt.request)

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
		})
	}
}

func TestV1_UpdateSystemL1KeyClaim(t *testing.T) {
	tests := []struct {
		name           string
		request        *system.UpdateSystemL1KeyClaimRequest
		mockResponse   *systemgrpc.UpdateSystemL1KeyClaimResponse
		mockError      error
		expectedError  error
		expectSuccess  bool
	}{
		{
			name: "successful update",
			request: &system.UpdateSystemL1KeyClaimRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				TenantID:   "tenant-456",
				L1KeyClaim: true,
			},
			mockResponse: &systemgrpc.UpdateSystemL1KeyClaimResponse{
				Success: true,
			},
			expectSuccess: true,
		},
		{
			name:          "nil request",
			request:       nil,
			expectedError: system.NewValidationError("request", "request cannot be nil"),
		},
		{
			name: "missing region",
			request: &system.UpdateSystemL1KeyClaimRequest{
				ExternalID: "ext-123",
				TenantID:   "tenant-456",
			},
			expectedError: system.NewValidationError("Region", "region is required"),
		},
		{
			name: "missing external ID",
			request: &system.UpdateSystemL1KeyClaimRequest{
				Region:   "us-east-1",
				TenantID: "tenant-456",
			},
			expectedError: system.NewValidationError("ExternalID", "external_id is required"),
		},
		{
			name: "missing tenant ID",
			request: &system.UpdateSystemL1KeyClaimRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
			},
			expectedError: system.NewValidationError("TenantID", "tenant_id is required"),
		},
		{
			name: "key claim already active",
			request: &system.UpdateSystemL1KeyClaimRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				TenantID:   "tenant-456",
				L1KeyClaim: true,
			},
			mockError:     status.Error(codes.FailedPrecondition, "key claim is already active"),
			expectedError: system.ErrL1KeyClaimAlreadyActive,
		},
		{
			name: "key claim already inactive",
			request: &system.UpdateSystemL1KeyClaimRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				TenantID:   "tenant-456",
				L1KeyClaim: false,
			},
			mockError:     status.Error(codes.FailedPrecondition, "key claim is already inactive"),
			expectedError: system.ErrL1KeyClaimAlreadyInactive,
		},
		{
			name: "system not linked to tenant",
			request: &system.UpdateSystemL1KeyClaimRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.FailedPrecondition, "system not linked to the tenant"),
			expectedError: system.ErrSystemNotLinkedToTenant,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockSystemClient{
				updateSystemL1KeyClaimFunc: func(ctx context.Context, req *systemgrpc.UpdateSystemL1KeyClaimRequest) (*systemgrpc.UpdateSystemL1KeyClaimResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
			resp, err := v1.UpdateSystemL1KeyClaim(context.Background(), tt.request)

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

func TestV1_DeleteSystem(t *testing.T) {
	tests := []struct {
		name           string
		request        *system.DeleteSystemRequest
		mockResponse   *systemgrpc.DeleteSystemResponse
		mockError      error
		expectedError  error
		expectSuccess  bool
	}{
		{
			name: "successful deletion",
			request: &system.DeleteSystemRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
			},
			mockResponse: &systemgrpc.DeleteSystemResponse{
				Success: true,
			},
			expectSuccess: true,
		},
		{
			name:          "nil request",
			request:       nil,
			expectedError: system.NewValidationError("request", "request cannot be nil"),
		},
		{
			name: "missing region",
			request: &system.DeleteSystemRequest{
				ExternalID: "ext-123",
			},
			expectedError: system.NewValidationError("Region", "region is required"),
		},
		{
			name: "missing external ID",
			request: &system.DeleteSystemRequest{
				Region: "us-east-1",
			},
			expectedError: system.NewValidationError("ExternalID", "external_id is required"),
		},
		{
			name: "system not found",
			request: &system.DeleteSystemRequest{
				Region:     "us-east-1",
				ExternalID: "non-existent",
			},
			mockError:     status.Error(codes.NotFound, "system not found"),
			expectedError: system.ErrSystemNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockSystemClient{
				deleteSystemFunc: func(ctx context.Context, req *systemgrpc.DeleteSystemRequest) (*systemgrpc.DeleteSystemResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
			resp, err := v1.DeleteSystem(context.Background(), tt.request)

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

func TestMapProtoToSystemType(t *testing.T) {
	tests := []struct {
		name      string
		protoType string
		expected  system.SystemType
	}{
		{
			name:      "keystore",
			protoType: "KEYSTORE",
			expected:  system.SystemTypeKeystore,
		},
		{
			name:      "application",
			protoType: "APPLICATION",
			expected:  system.SystemTypeApplication,
		},
		{
			name:      "lowercase keystore",
			protoType: "keystore",
			expected:  system.SystemTypeKeystore,
		},
		{
			name:      "unknown type",
			protoType: "UNKNOWN",
			expected:  system.SystemTypeUnspecified,
		},
		{
			name:      "empty string",
			protoType: "",
			expected:  system.SystemTypeUnspecified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapProtoToSystemType(tt.protoType)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMapSystemTypeToProto(t *testing.T) {
	tests := []struct {
		name     string
		sysType  system.SystemType
		expected string
	}{
		{
			name:     "keystore",
			sysType:  system.SystemTypeKeystore,
			expected: "KEYSTORE",
		},
		{
			name:     "application",
			sysType:  system.SystemTypeApplication,
			expected: "APPLICATION",
		},
		{
			name:     "unspecified",
			sysType:  system.SystemTypeUnspecified,
			expected: "UNSPECIFIED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapSystemTypeToProto(tt.sysType)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestConvertGRPCError(t *testing.T) {
	tests := []struct {
		name          string
		inputError    error
		expectedError error
		expectContains string
	}{
		{
			name:          "nil error",
			inputError:    nil,
			expectedError: nil,
		},
		{
			name:          "system not found",
			inputError:    status.Error(codes.NotFound, "system not found"),
			expectedError: system.ErrSystemNotFound,
		},
		{
			name:          "already exists",
			inputError:    status.Error(codes.AlreadyExists, "already exists"),
			expectedError: system.ErrSystemAlreadyExists,
		},
		{
			name:          "invalid region",
			inputError:    status.Error(codes.InvalidArgument, "invalid region"),
			expectedError: system.ErrInvalidRegion,
		},
		{
			name:          "invalid external ID",
			inputError:    status.Error(codes.InvalidArgument, "invalid external ID"),
			expectedError: system.ErrInvalidExternalID,
		},
		{
			name:          "invalid tenant ID",
			inputError:    status.Error(codes.InvalidArgument, "invalid tenant ID"),
			expectedError: system.ErrInvalidTenantID,
		},
		{
			name:          "invalid type",
			inputError:    status.Error(codes.InvalidArgument, "invalid type"),
			expectedError: system.ErrInvalidSystemType,
		},
		{
			name:           "invalid argument without keyword",
			inputError:     status.Error(codes.InvalidArgument, "something wrong"),
			expectContains: "invalid argument",
		},
		{
			name:          "key claim already active",
			inputError:    status.Error(codes.FailedPrecondition, "key claim is already active"),
			expectedError: system.ErrL1KeyClaimAlreadyActive,
		},
		{
			name:          "key claim already inactive",
			inputError:    status.Error(codes.FailedPrecondition, "key claim is already inactive"),
			expectedError: system.ErrL1KeyClaimAlreadyInactive,
		},
		{
			name:          "system not linked to tenant",
			inputError:    status.Error(codes.FailedPrecondition, "not linked to the tenant"),
			expectedError: system.ErrSystemNotLinkedToTenant,
		},
		{
			name:           "failed precondition without keyword",
			inputError:     status.Error(codes.FailedPrecondition, "something wrong"),
			expectContains: "failed precondition",
		},
		{
			name:           "non-grpc error",
			inputError:     errors.New("network error"),
			expectedError:  errors.New("network error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertGRPCError(tt.inputError)

			if tt.expectedError != nil {
				if result == nil {
					t.Fatalf("expected error %v, got nil", tt.expectedError)
				}
				if result.Error() != tt.expectedError.Error() {
					t.Errorf("expected error %v, got %v", tt.expectedError, result)
				}
			} else if tt.expectContains != "" {
				if result == nil {
					t.Fatal("expected error, got nil")
				}
				if !contains(result.Error(), tt.expectContains) {
					t.Errorf("expected error to contain %q, got %q", tt.expectContains, result.Error())
				}
			} else if result != nil {
				t.Errorf("expected nil error, got %v", result)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsMiddle(s, substr)
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
