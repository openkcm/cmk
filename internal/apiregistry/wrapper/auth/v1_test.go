package auth_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"

	authapi "github.com/openkcm/cmk/internal/apiregistry/api/auth"
	apierrors "github.com/openkcm/cmk/internal/apiregistry/errors"
	"github.com/openkcm/cmk/internal/apiregistry/wrapper/auth"
)

// mockAuthClient is a mock implementation of authgrpc.ServiceClient
type mockAuthClient struct {
	authgrpc.ServiceClient // embed to satisfy interface

	applyAuthFunc  func(ctx context.Context, req *authgrpc.ApplyAuthRequest) (*authgrpc.ApplyAuthResponse, error)
	getAuthFunc    func(ctx context.Context, req *authgrpc.GetAuthRequest) (*authgrpc.GetAuthResponse, error)
	listAuthsFunc  func(ctx context.Context, req *authgrpc.ListAuthsRequest) (*authgrpc.ListAuthsResponse, error)
	removeAuthFunc func(ctx context.Context, req *authgrpc.RemoveAuthRequest) (*authgrpc.RemoveAuthResponse, error)
}

func (m *mockAuthClient) ApplyAuth(ctx context.Context, req *authgrpc.ApplyAuthRequest, opts ...grpc.CallOption) (*authgrpc.ApplyAuthResponse, error) {
	return m.applyAuthFunc(ctx, req)
}

func (m *mockAuthClient) GetAuth(ctx context.Context, req *authgrpc.GetAuthRequest, opts ...grpc.CallOption) (*authgrpc.GetAuthResponse, error) {
	return m.getAuthFunc(ctx, req)
}

func (m *mockAuthClient) ListAuths(ctx context.Context, req *authgrpc.ListAuthsRequest, opts ...grpc.CallOption) (*authgrpc.ListAuthsResponse, error) {
	return m.listAuthsFunc(ctx, req)
}

func (m *mockAuthClient) RemoveAuth(ctx context.Context, req *authgrpc.RemoveAuthRequest, opts ...grpc.CallOption) (*authgrpc.RemoveAuthResponse, error) {
	return m.removeAuthFunc(ctx, req)
}

func TestNewV1(t *testing.T) {
	mockClient := &mockAuthClient{}
	v1 := auth.NewV1(mockClient)

	if v1 == nil {
		t.Fatal("expected non-nil V1 instance")
	}
}

func TestV1_ApplyAuth(t *testing.T) {
	tests := []struct {
		name          string
		request       *authapi.ApplyAuthRequest
		mockResponse  *authgrpc.ApplyAuthResponse
		mockError     error
		expectedError error
		expectSuccess bool
	}{
		{
			name: "successful apply",
			request: &authapi.ApplyAuthRequest{
				ExternalID: "ext-123",
				TenantID:   "tenant-456",
				Type:       "OAUTH2",
				Properties: map[string]string{"client_id": "abc", "client_secret": "xyz"},
			},
			mockResponse:  &authgrpc.ApplyAuthResponse{Success: true},
			expectSuccess: true,
		},
		{
			name:          "nil request",
			request:       nil,
			expectedError: apierrors.NewValidationError("request", "request cannot be nil"),
		},
		{
			name: "missing external_id",
			request: &authapi.ApplyAuthRequest{
				TenantID:   "tenant-456",
				Type:       "OAUTH2",
				Properties: map[string]string{"client_id": "abc"},
			},
			expectedError: apierrors.NewValidationError("ExternalID", "external_id is required"),
		},
		{
			name: "missing tenant_id",
			request: &authapi.ApplyAuthRequest{
				ExternalID: "ext-123",
				Type:       "OAUTH2",
				Properties: map[string]string{"client_id": "abc"},
			},
			expectedError: apierrors.NewValidationError("TenantID", "tenant_id is required"),
		},
		{
			name: "missing type",
			request: &authapi.ApplyAuthRequest{
				ExternalID: "ext-123",
				TenantID:   "tenant-456",
				Properties: map[string]string{"client_id": "abc"},
			},
			expectedError: apierrors.NewValidationError("Type", "type is required"),
		},
		{
			name: "empty properties",
			request: &authapi.ApplyAuthRequest{
				ExternalID: "ext-123",
				TenantID:   "tenant-456",
				Type:       "OAUTH2",
				Properties: map[string]string{},
			},
			expectedError: apierrors.NewValidationError("Properties", "at least one property is required"),
		},
		{
			name: "auth already exists",
			request: &authapi.ApplyAuthRequest{
				ExternalID: "ext-123",
				TenantID:   "tenant-456",
				Type:       "OAUTH2",
				Properties: map[string]string{"client_id": "abc"},
			},
			mockError:     status.Error(codes.AlreadyExists, "auth already exists"),
			expectedError: apierrors.ErrAuthAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockAuthClient{
				applyAuthFunc: func(ctx context.Context, req *authgrpc.ApplyAuthRequest) (*authgrpc.ApplyAuthResponse, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}
					return tt.mockResponse, nil
				},
			}

			v1 := auth.NewV1(mockClient)
			resp, err := v1.ApplyAuth(context.Background(), tt.request)

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

func TestV1_GetAuth(t *testing.T) {
	tests := []struct {
		name          string
		request       *authapi.GetAuthRequest
		mockResponse  *authgrpc.GetAuthResponse
		mockError     error
		expectedError error
		expectAuth    bool
	}{
		{
			name: "successful get",
			request: &authapi.GetAuthRequest{
				ExternalID: "ext-123",
			},
			mockResponse: &authgrpc.GetAuthResponse{
				Auth: &authgrpc.Auth{
					ExternalId: "ext-123",
					TenantId:   "tenant-456",
					Type:       "OAUTH2",
					Properties: map[string]string{"client_id": "abc"},
					Status:     authgrpc.AuthStatus_AUTH_STATUS_APPLIED,
				},
			},
			expectAuth: true,
		},
		{
			name:          "nil request",
			request:       nil,
			expectedError: apierrors.NewValidationError("request", "request cannot be nil"),
		},
		{
			name: "missing external_id",
			request: &authapi.GetAuthRequest{
				ExternalID: "",
			},
			expectedError: apierrors.NewValidationError("ExternalID", "external_id is required"),
		},
		{
			name: "auth not found",
			request: &authapi.GetAuthRequest{
				ExternalID: "non-existent",
			},
			mockError:     status.Error(codes.NotFound, "auth not found"),
			expectedError: apierrors.ErrAuthNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockAuthClient{
				getAuthFunc: func(ctx context.Context, req *authgrpc.GetAuthRequest) (*authgrpc.GetAuthResponse, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}
					return tt.mockResponse, nil
				},
			}

			v1 := auth.NewV1(mockClient)
			resp, err := v1.GetAuth(context.Background(), tt.request)

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

			if tt.expectAuth && resp.Auth == nil {
				t.Error("expected auth to be non-nil")
			}
		})
	}
}

func TestV1_ListAuths(t *testing.T) {
	tests := []struct {
		name          string
		request       *authapi.ListAuthsRequest
		mockResponse  *authgrpc.ListAuthsResponse
		mockError     error
		expectedError error
		expectedCount int
	}{
		{
			name: "successful list",
			request: &authapi.ListAuthsRequest{
				TenantID: "tenant-456",
				Limit:    10,
			},
			mockResponse: &authgrpc.ListAuthsResponse{
				Auth: []*authgrpc.Auth{
					{
						ExternalId: "ext-123",
						TenantId:   "tenant-456",
						Type:       "OAUTH2",
						Properties: map[string]string{"client_id": "abc"},
						Status:     authgrpc.AuthStatus_AUTH_STATUS_APPLIED,
					},
					{
						ExternalId: "ext-456",
						TenantId:   "tenant-456",
						Type:       "API_KEY",
						Properties: map[string]string{"api_key": "xyz"},
						Status:     authgrpc.AuthStatus_AUTH_STATUS_APPLIED,
					},
				},
				NextPageToken: "next-token",
			},
			expectedCount: 2,
		},
		{
			name:          "nil request",
			request:       nil,
			expectedError: apierrors.NewValidationError("request", "request cannot be nil"),
		},
		{
			name: "invalid limit too low",
			request: &authapi.ListAuthsRequest{
				TenantID: "tenant-456",
				Limit:    -1,
			},
			expectedError: apierrors.ErrAuthInvalidLimit,
		},
		{
			name: "invalid limit too high",
			request: &authapi.ListAuthsRequest{
				TenantID: "tenant-456",
				Limit:    1001,
			},
			expectedError: apierrors.ErrAuthInvalidLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockAuthClient{
				listAuthsFunc: func(ctx context.Context, req *authgrpc.ListAuthsRequest) (*authgrpc.ListAuthsResponse, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}
					return tt.mockResponse, nil
				},
			}

			v1 := auth.NewV1(mockClient)
			resp, err := v1.ListAuths(context.Background(), tt.request)

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

			if len(resp.Auths) != tt.expectedCount {
				t.Errorf("expected %d auths, got %d", tt.expectedCount, len(resp.Auths))
			}
		})
	}
}

func TestV1_RemoveAuth(t *testing.T) {
	tests := []struct {
		name          string
		request       *authapi.RemoveAuthRequest
		mockResponse  *authgrpc.RemoveAuthResponse
		mockError     error
		expectedError error
		expectSuccess bool
	}{
		{
			name: "successful remove",
			request: &authapi.RemoveAuthRequest{
				ExternalID: "ext-123",
			},
			mockResponse:  &authgrpc.RemoveAuthResponse{Success: true},
			expectSuccess: true,
		},
		{
			name:          "nil request",
			request:       nil,
			expectedError: apierrors.NewValidationError("request", "request cannot be nil"),
		},
		{
			name: "missing external_id",
			request: &authapi.RemoveAuthRequest{
				ExternalID: "",
			},
			expectedError: apierrors.NewValidationError("ExternalID", "external_id is required"),
		},
		{
			name: "auth not found",
			request: &authapi.RemoveAuthRequest{
				ExternalID: "non-existent",
			},
			mockError:     status.Error(codes.NotFound, "auth not found"),
			expectedError: apierrors.ErrAuthNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockAuthClient{
				removeAuthFunc: func(ctx context.Context, req *authgrpc.RemoveAuthRequest) (*authgrpc.RemoveAuthResponse, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}
					return tt.mockResponse, nil
				},
			}

			v1 := auth.NewV1(mockClient)
			resp, err := v1.RemoveAuth(context.Background(), tt.request)

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
