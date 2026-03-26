package oidcmapping

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	oidcmappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"

	oidcmappingapi "github.com/openkcm/cmk/internal/apiregistry/api/oidcmapping"
)

// mockOIDCMappingClient is a mock implementation of oidcmappinggrpc.ServiceClient
type mockOIDCMappingClient struct {
	applyOIDCMappingFunc   func(ctx context.Context, req *oidcmappinggrpc.ApplyOIDCMappingRequest) (*oidcmappinggrpc.ApplyOIDCMappingResponse, error)
	removeOIDCMappingFunc  func(ctx context.Context, req *oidcmappinggrpc.RemoveOIDCMappingRequest) (*oidcmappinggrpc.RemoveOIDCMappingResponse, error)
	blockOIDCMappingFunc   func(ctx context.Context, req *oidcmappinggrpc.BlockOIDCMappingRequest) (*oidcmappinggrpc.BlockOIDCMappingResponse, error)
	unblockOIDCMappingFunc func(ctx context.Context, req *oidcmappinggrpc.UnblockOIDCMappingRequest) (*oidcmappinggrpc.UnblockOIDCMappingResponse, error)
}

func (m *mockOIDCMappingClient) ApplyOIDCMapping(ctx context.Context, req *oidcmappinggrpc.ApplyOIDCMappingRequest, opts ...grpc.CallOption) (*oidcmappinggrpc.ApplyOIDCMappingResponse, error) {
	return m.applyOIDCMappingFunc(ctx, req)
}

func (m *mockOIDCMappingClient) RemoveOIDCMapping(ctx context.Context, req *oidcmappinggrpc.RemoveOIDCMappingRequest, opts ...grpc.CallOption) (*oidcmappinggrpc.RemoveOIDCMappingResponse, error) {
	return m.removeOIDCMappingFunc(ctx, req)
}

func (m *mockOIDCMappingClient) BlockOIDCMapping(ctx context.Context, req *oidcmappinggrpc.BlockOIDCMappingRequest, opts ...grpc.CallOption) (*oidcmappinggrpc.BlockOIDCMappingResponse, error) {
	return m.blockOIDCMappingFunc(ctx, req)
}

func (m *mockOIDCMappingClient) UnblockOIDCMapping(ctx context.Context, req *oidcmappinggrpc.UnblockOIDCMappingRequest, opts ...grpc.CallOption) (*oidcmappinggrpc.UnblockOIDCMappingResponse, error) {
	return m.unblockOIDCMappingFunc(ctx, req)
}

func TestNewV1(t *testing.T) {
	mockClient := &mockOIDCMappingClient{}
	v1 := NewV1(mockClient)

	if v1 == nil {
		t.Fatal("expected non-nil V1 instance")
	}
	if v1.client == nil {
		t.Error("expected client to be set")
	}
}

//nolint:cyclop // test function with many test cases
func TestV1_ApplyOIDCMapping(t *testing.T) {
	jwksURI := "https://example.com/.well-known/jwks.json"
	clientID := "client-123"
	message := "success message"

	tests := []struct {
		name          string
		request       *oidcmappingapi.ApplyOIDCMappingRequest
		mockResponse  *oidcmappinggrpc.ApplyOIDCMappingResponse
		mockError     error
		expectedError error
		expectSuccess bool
		expectMessage *string
	}{
		{
			name: "successful apply with all fields",
			request: &oidcmappingapi.ApplyOIDCMappingRequest{
				TenantID:   "tenant-123",
				Issuer:     "https://issuer.example.com",
				JwksURI:    &jwksURI,
				Audiences:  []string{"aud1", "aud2"},
				ClientID:   &clientID,
				Properties: map[string]string{"key": "value"},
			},
			mockResponse: &oidcmappinggrpc.ApplyOIDCMappingResponse{
				Success: true,
				Message: &message,
			},
			expectSuccess: true,
			expectMessage: &message,
		},
		{
			name: "successful apply with required fields only",
			request: &oidcmappingapi.ApplyOIDCMappingRequest{
				TenantID: "tenant-123",
				Issuer:   "https://issuer.example.com",
			},
			mockResponse: &oidcmappinggrpc.ApplyOIDCMappingResponse{
				Success: true,
			},
			expectSuccess: true,
		},
		{
			name: "missing tenant ID",
			request: &oidcmappingapi.ApplyOIDCMappingRequest{
				TenantID: "",
				Issuer:   "https://issuer.example.com",
			},
			expectedError: oidcmappingapi.NewValidationError("TenantID", "tenant ID is required"),
		},
		{
			name: "missing issuer",
			request: &oidcmappingapi.ApplyOIDCMappingRequest{
				TenantID: "tenant-123",
				Issuer:   "",
			},
			expectedError: oidcmappingapi.NewValidationError("Issuer", "issuer is required"),
		},
		{
			name: "mapping already exists",
			request: &oidcmappingapi.ApplyOIDCMappingRequest{
				TenantID: "tenant-123",
				Issuer:   "https://issuer.example.com",
			},
			mockError:     status.Error(codes.AlreadyExists, "mapping already exists"),
			expectedError: oidcmappingapi.ErrOIDCMappingAlreadyExists,
		},
		{
			name: "invalid tenant ID",
			request: &oidcmappingapi.ApplyOIDCMappingRequest{
				TenantID: "tenant-123",
				Issuer:   "https://issuer.example.com",
			},
			mockError:     status.Error(codes.InvalidArgument, "tenant ID is invalid"),
			expectedError: oidcmappingapi.ErrInvalidTenantID,
		},
		{
			name: "invalid issuer",
			request: &oidcmappingapi.ApplyOIDCMappingRequest{
				TenantID: "tenant-123",
				Issuer:   "https://issuer.example.com",
			},
			mockError:     status.Error(codes.InvalidArgument, "issuer is invalid"),
			expectedError: oidcmappingapi.ErrInvalidIssuer,
		},
		{
			name: "invalid jwks URI",
			request: &oidcmappingapi.ApplyOIDCMappingRequest{
				TenantID: "tenant-123",
				Issuer:   "https://issuer.example.com",
			},
			mockError:     status.Error(codes.InvalidArgument, "jwks URI is invalid"),
			expectedError: oidcmappingapi.ErrInvalidJwksURI,
		},
		{
			name: "invalid audience",
			request: &oidcmappingapi.ApplyOIDCMappingRequest{
				TenantID: "tenant-123",
				Issuer:   "https://issuer.example.com",
			},
			mockError:     status.Error(codes.InvalidArgument, "audience is invalid"),
			expectedError: oidcmappingapi.ErrInvalidAudiences,
		},
		{
			name: "invalid client ID",
			request: &oidcmappingapi.ApplyOIDCMappingRequest{
				TenantID: "tenant-123",
				Issuer:   "https://issuer.example.com",
			},
			mockError:     status.Error(codes.InvalidArgument, "client ID is invalid"),
			expectedError: oidcmappingapi.ErrInvalidClientID,
		},
		{
			name: "operation failed",
			request: &oidcmappingapi.ApplyOIDCMappingRequest{
				TenantID: "tenant-123",
				Issuer:   "https://issuer.example.com",
			},
			mockError:     status.Error(codes.Internal, "internal error"),
			expectedError: oidcmappingapi.ErrOperationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockOIDCMappingClient{
				applyOIDCMappingFunc: func(ctx context.Context, req *oidcmappinggrpc.ApplyOIDCMappingRequest) (*oidcmappinggrpc.ApplyOIDCMappingResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
			resp, err := v1.ApplyOIDCMapping(context.Background(), tt.request)

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

			if tt.expectMessage != nil {
				if resp.Message == nil {
					t.Error("expected message to be set")
				} else if *resp.Message != *tt.expectMessage {
					t.Errorf("expected message=%s, got %s", *tt.expectMessage, *resp.Message)
				}
			} else {
				if resp.Message != nil {
					t.Errorf("expected message to be nil, got %s", *resp.Message)
				}
			}
		})
	}
}

//nolint:cyclop // test function with many test cases
func TestV1_RemoveOIDCMapping(t *testing.T) {
	message := "removed successfully"

	tests := []struct {
		name          string
		request       *oidcmappingapi.RemoveOIDCMappingRequest
		mockResponse  *oidcmappinggrpc.RemoveOIDCMappingResponse
		mockError     error
		expectedError error
		expectSuccess bool
		expectMessage *string
	}{
		{
			name: "successful remove with message",
			request: &oidcmappingapi.RemoveOIDCMappingRequest{
				TenantID: "tenant-123",
			},
			mockResponse: &oidcmappinggrpc.RemoveOIDCMappingResponse{
				Success: true,
				Message: &message,
			},
			expectSuccess: true,
			expectMessage: &message,
		},
		{
			name: "successful remove without message",
			request: &oidcmappingapi.RemoveOIDCMappingRequest{
				TenantID: "tenant-123",
			},
			mockResponse: &oidcmappinggrpc.RemoveOIDCMappingResponse{
				Success: true,
			},
			expectSuccess: true,
		},
		{
			name: "missing tenant ID",
			request: &oidcmappingapi.RemoveOIDCMappingRequest{
				TenantID: "",
			},
			expectedError: oidcmappingapi.NewValidationError("TenantID", "tenant ID is required"),
		},
		{
			name: "mapping not found",
			request: &oidcmappingapi.RemoveOIDCMappingRequest{
				TenantID: "tenant-123",
			},
			mockError:     status.Error(codes.NotFound, "mapping not found"),
			expectedError: oidcmappingapi.ErrOIDCMappingNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockOIDCMappingClient{
				removeOIDCMappingFunc: func(ctx context.Context, req *oidcmappinggrpc.RemoveOIDCMappingRequest) (*oidcmappinggrpc.RemoveOIDCMappingResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
			resp, err := v1.RemoveOIDCMapping(context.Background(), tt.request)

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

			if tt.expectMessage != nil {
				if resp.Message == nil {
					t.Error("expected message to be set")
				} else if *resp.Message != *tt.expectMessage {
					t.Errorf("expected message=%s, got %s", *tt.expectMessage, *resp.Message)
				}
			} else {
				if resp.Message != nil {
					t.Errorf("expected message to be nil, got %s", *resp.Message)
				}
			}
		})
	}
}

func TestV1_BlockOIDCMapping(t *testing.T) {
	tests := []struct {
		name          string
		request       *oidcmappingapi.BlockOIDCMappingRequest
		mockResponse  *oidcmappinggrpc.BlockOIDCMappingResponse
		mockError     error
		expectedError error
		expectSuccess bool
	}{
		{
			name: "successful block",
			request: &oidcmappingapi.BlockOIDCMappingRequest{
				TenantID: "tenant-123",
			},
			mockResponse: &oidcmappinggrpc.BlockOIDCMappingResponse{
				Success: true,
			},
			expectSuccess: true,
		},
		{
			name: "missing tenant ID",
			request: &oidcmappingapi.BlockOIDCMappingRequest{
				TenantID: "",
			},
			expectedError: oidcmappingapi.NewValidationError("TenantID", "tenant ID is required"),
		},
		{
			name: "already blocked",
			request: &oidcmappingapi.BlockOIDCMappingRequest{
				TenantID: "tenant-123",
			},
			mockError:     status.Error(codes.FailedPrecondition, "mapping is already blocked"),
			expectedError: oidcmappingapi.ErrOIDCMappingAlreadyBlocked,
		},
		{
			name: "mapping not found",
			request: &oidcmappingapi.BlockOIDCMappingRequest{
				TenantID: "tenant-123",
			},
			mockError:     status.Error(codes.NotFound, "mapping not found"),
			expectedError: oidcmappingapi.ErrOIDCMappingNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockOIDCMappingClient{
				blockOIDCMappingFunc: func(ctx context.Context, req *oidcmappinggrpc.BlockOIDCMappingRequest) (*oidcmappinggrpc.BlockOIDCMappingResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
			resp, err := v1.BlockOIDCMapping(context.Background(), tt.request)

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

func TestV1_UnblockOIDCMapping(t *testing.T) {
	tests := []struct {
		name          string
		request       *oidcmappingapi.UnblockOIDCMappingRequest
		mockResponse  *oidcmappinggrpc.UnblockOIDCMappingResponse
		mockError     error
		expectedError error
		expectSuccess bool
	}{
		{
			name: "successful unblock",
			request: &oidcmappingapi.UnblockOIDCMappingRequest{
				TenantID: "tenant-123",
			},
			mockResponse: &oidcmappinggrpc.UnblockOIDCMappingResponse{
				Success: true,
			},
			expectSuccess: true,
		},
		{
			name: "missing tenant ID",
			request: &oidcmappingapi.UnblockOIDCMappingRequest{
				TenantID: "",
			},
			expectedError: oidcmappingapi.NewValidationError("TenantID", "tenant ID is required"),
		},
		{
			name: "not blocked",
			request: &oidcmappingapi.UnblockOIDCMappingRequest{
				TenantID: "tenant-123",
			},
			mockError:     status.Error(codes.FailedPrecondition, "mapping is not blocked"),
			expectedError: oidcmappingapi.ErrOIDCMappingNotBlocked,
		},
		{
			name: "mapping not found",
			request: &oidcmappingapi.UnblockOIDCMappingRequest{
				TenantID: "tenant-123",
			},
			mockError:     status.Error(codes.NotFound, "mapping not found"),
			expectedError: oidcmappingapi.ErrOIDCMappingNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockOIDCMappingClient{
				unblockOIDCMappingFunc: func(ctx context.Context, req *oidcmappinggrpc.UnblockOIDCMappingRequest) (*oidcmappinggrpc.UnblockOIDCMappingResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
			resp, err := v1.UnblockOIDCMapping(context.Background(), tt.request)

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

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "exact match",
			s:        "tenant",
			substr:   "tenant",
			expected: true,
		},
		{
			name:     "substring in middle",
			s:        "the tenant is",
			substr:   "tenant",
			expected: true,
		},
		{
			name:     "no match",
			s:        "issuer",
			substr:   "tenant",
			expected: false,
		},
		{
			name:     "empty string",
			s:        "",
			substr:   "tenant",
			expected: false,
		},
		{
			name:     "empty substring",
			s:        "tenant",
			substr:   "",
			expected: true,
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
			expectedError: oidcmappingapi.ErrOIDCMappingNotFound,
		},
		{
			name:          "already exists",
			inputError:    status.Error(codes.AlreadyExists, "already exists"),
			expectedError: oidcmappingapi.ErrOIDCMappingAlreadyExists,
		},
		{
			name:          "invalid argument - tenant",
			inputError:    status.Error(codes.InvalidArgument, "invalid tenant"),
			expectedError: oidcmappingapi.ErrInvalidTenantID,
		},
		{
			name:          "invalid argument - issuer",
			inputError:    status.Error(codes.InvalidArgument, "invalid issuer"),
			expectedError: oidcmappingapi.ErrInvalidIssuer,
		},
		{
			name:          "invalid argument - jwks",
			inputError:    status.Error(codes.InvalidArgument, "invalid jwks"),
			expectedError: oidcmappingapi.ErrInvalidJwksURI,
		},
		{
			name:          "invalid argument - audience",
			inputError:    status.Error(codes.InvalidArgument, "invalid audience"),
			expectedError: oidcmappingapi.ErrInvalidAudiences,
		},
		{
			name:          "invalid argument - client",
			inputError:    status.Error(codes.InvalidArgument, "invalid client"),
			expectedError: oidcmappingapi.ErrInvalidClientID,
		},
		{
			name:          "failed precondition - already blocked",
			inputError:    status.Error(codes.FailedPrecondition, "already blocked"),
			expectedError: oidcmappingapi.ErrOIDCMappingAlreadyBlocked,
		},
		{
			name:          "failed precondition - not blocked",
			inputError:    status.Error(codes.FailedPrecondition, "not blocked"),
			expectedError: oidcmappingapi.ErrOIDCMappingNotBlocked,
		},
		{
			name:          "internal error",
			inputError:    status.Error(codes.Internal, "internal error"),
			expectedError: oidcmappingapi.ErrOperationFailed,
		},
		{
			name:          "non-grpc error",
			inputError:    errors.New("network error"), //nolint:err113 // test error
			expectedError: oidcmappingapi.ErrOperationFailed,
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
