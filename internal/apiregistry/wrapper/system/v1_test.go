package system_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"

	systemapi "github.com/openkcm/cmk/internal/apiregistry/api/system"
	apierrors "github.com/openkcm/cmk/internal/apiregistry/errors"
	"github.com/openkcm/cmk/internal/apiregistry/wrapper/system"
)

// mockSystemClient is a mock implementation of systemgrpc.ServiceClient
type mockSystemClient struct {
	systemgrpc.ServiceClient // embed to satisfy interface

	listSystemsFunc            func(ctx context.Context, req *systemgrpc.ListSystemsRequest) (*systemgrpc.ListSystemsResponse, error)
	registerSystemFunc         func(ctx context.Context, req *systemgrpc.RegisterSystemRequest) (*systemgrpc.RegisterSystemResponse, error)
	updateSystemL1KeyClaimFunc func(ctx context.Context, req *systemgrpc.UpdateSystemL1KeyClaimRequest) (*systemgrpc.UpdateSystemL1KeyClaimResponse, error)
	deleteSystemFunc           func(ctx context.Context, req *systemgrpc.DeleteSystemRequest) (*systemgrpc.DeleteSystemResponse, error)
	updateSystemStatusFunc     func(ctx context.Context, req *systemgrpc.UpdateSystemStatusRequest) (*systemgrpc.UpdateSystemStatusResponse, error)
	setSystemLabelsFunc        func(ctx context.Context, req *systemgrpc.SetSystemLabelsRequest) (*systemgrpc.SetSystemLabelsResponse, error)
	removeSystemLabelsFunc     func(ctx context.Context, req *systemgrpc.RemoveSystemLabelsRequest) (*systemgrpc.RemoveSystemLabelsResponse, error)
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

func (m *mockSystemClient) UpdateSystemStatus(ctx context.Context, req *systemgrpc.UpdateSystemStatusRequest, opts ...grpc.CallOption) (*systemgrpc.UpdateSystemStatusResponse, error) {
	return m.updateSystemStatusFunc(ctx, req)
}

func (m *mockSystemClient) SetSystemLabels(ctx context.Context, req *systemgrpc.SetSystemLabelsRequest, opts ...grpc.CallOption) (*systemgrpc.SetSystemLabelsResponse, error) {
	return m.setSystemLabelsFunc(ctx, req)
}

func (m *mockSystemClient) RemoveSystemLabels(ctx context.Context, req *systemgrpc.RemoveSystemLabelsRequest, opts ...grpc.CallOption) (*systemgrpc.RemoveSystemLabelsResponse, error) {
	return m.removeSystemLabelsFunc(ctx, req)
}

func TestNewV1(t *testing.T) {
	mockClient := &mockSystemClient{}
	v1 := system.NewV1(mockClient)

	if v1 == nil {
		t.Fatal("expected non-nil V1 instance")
	}
}

func TestV1_ListSystems(t *testing.T) {
	tests := []struct {
		name          string
		request       *systemapi.ListSystemsRequest
		mockResponse  *systemgrpc.ListSystemsResponse
		mockError     error
		expectedError error
		expectedCount int
	}{
		{
			name: "successful list",
			request: &systemapi.ListSystemsRequest{
				Region: "us-east-1",
				Limit:  10,
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
			name:          "nil request",
			request:       nil,
			expectedError: apierrors.NewValidationError("request", "request cannot be nil"),
		},
		{
			name: "invalid limit",
			request: &systemapi.ListSystemsRequest{
				Limit: -1,
			},
			expectedError: apierrors.ErrSystemInvalidLimit,
		},
		{
			name: "system not found",
			request: &systemapi.ListSystemsRequest{
				Region:     "us-east-1",
				ExternalID: "non-existent",
			},
			mockError:     status.Error(codes.NotFound, "system not found"),
			expectedError: apierrors.ErrSystemNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockSystemClient{
				listSystemsFunc: func(ctx context.Context, req *systemgrpc.ListSystemsRequest) (*systemgrpc.ListSystemsResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := system.NewV1(mockClient)
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
		name          string
		request       *systemapi.RegisterSystemRequest
		mockResponse  *systemgrpc.RegisterSystemResponse
		mockError     error
		expectedError error
	}{
		{
			name: "successful registration",
			request: &systemapi.RegisterSystemRequest{
				Region:        "us-east-1",
				ExternalID:    "ext-123",
				Type:          systemapi.SystemTypeKeystore,
				TenantID:      "tenant-456",
				L2KeyID:       "l2-key",
				HasL1KeyClaim: true,
			},
			mockResponse: &systemgrpc.RegisterSystemResponse{},
		},
		{
			name:          "nil request",
			request:       nil,
			expectedError: apierrors.NewValidationError("request", "request cannot be nil"),
		},
		{
			name: "missing region",
			request: &systemapi.RegisterSystemRequest{
				ExternalID: "ext-123",
				Type:       systemapi.SystemTypeKeystore,
				TenantID:   "tenant-456",
			},
			expectedError: apierrors.NewValidationError("Region", "region is required"),
		},
		{
			name: "missing external ID",
			request: &systemapi.RegisterSystemRequest{
				Region:   "us-east-1",
				Type:     systemapi.SystemTypeKeystore,
				TenantID: "tenant-456",
			},
			expectedError: apierrors.NewValidationError("ExternalID", "external_id is required"),
		},
		{
			name: "missing tenant ID",
			request: &systemapi.RegisterSystemRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				Type:       systemapi.SystemTypeKeystore,
			},
			expectedError: apierrors.NewValidationError("TenantID", "tenant_id is required"),
		},
		{
			name: "unspecified type",
			request: &systemapi.RegisterSystemRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				Type:       systemapi.SystemTypeUnspecified,
				TenantID:   "tenant-456",
			},
			expectedError: apierrors.NewValidationError("Type", "type must be specified"),
		},
		{
			name: "system already exists",
			request: &systemapi.RegisterSystemRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				Type:       systemapi.SystemTypeKeystore,
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.AlreadyExists, "system already exists"),
			expectedError: apierrors.ErrSystemAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockSystemClient{
				registerSystemFunc: func(ctx context.Context, req *systemgrpc.RegisterSystemRequest) (*systemgrpc.RegisterSystemResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := system.NewV1(mockClient)
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
		name          string
		request       *systemapi.UpdateSystemL1KeyClaimRequest
		mockResponse  *systemgrpc.UpdateSystemL1KeyClaimResponse
		mockError     error
		expectedError error
		expectSuccess bool
	}{
		{
			name: "successful update",
			request: &systemapi.UpdateSystemL1KeyClaimRequest{
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
			expectedError: apierrors.NewValidationError("request", "request cannot be nil"),
		},
		{
			name: "missing region",
			request: &systemapi.UpdateSystemL1KeyClaimRequest{
				ExternalID: "ext-123",
				TenantID:   "tenant-456",
			},
			expectedError: apierrors.NewValidationError("Region", "region is required"),
		},
		{
			name: "missing external ID",
			request: &systemapi.UpdateSystemL1KeyClaimRequest{
				Region:   "us-east-1",
				TenantID: "tenant-456",
			},
			expectedError: apierrors.NewValidationError("ExternalID", "external_id is required"),
		},
		{
			name: "missing tenant ID",
			request: &systemapi.UpdateSystemL1KeyClaimRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
			},
			expectedError: apierrors.NewValidationError("TenantID", "tenant_id is required"),
		},
		{
			name: "key claim already active",
			request: &systemapi.UpdateSystemL1KeyClaimRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				TenantID:   "tenant-456",
				L1KeyClaim: true,
			},
			mockError:     status.Error(codes.FailedPrecondition, "key claim is already active"),
			expectedError: apierrors.ErrL1KeyClaimAlreadyActive,
		},
		{
			name: "key claim already inactive",
			request: &systemapi.UpdateSystemL1KeyClaimRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				TenantID:   "tenant-456",
				L1KeyClaim: false,
			},
			mockError:     status.Error(codes.FailedPrecondition, "key claim is already inactive"),
			expectedError: apierrors.ErrL1KeyClaimAlreadyInactive,
		},
		{
			name: "system not linked to tenant",
			request: &systemapi.UpdateSystemL1KeyClaimRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				TenantID:   "tenant-456",
			},
			mockError:     status.Error(codes.FailedPrecondition, "system not linked to the tenant"),
			expectedError: apierrors.ErrSystemNotLinkedToTenant,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockSystemClient{
				updateSystemL1KeyClaimFunc: func(ctx context.Context, req *systemgrpc.UpdateSystemL1KeyClaimRequest) (*systemgrpc.UpdateSystemL1KeyClaimResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := system.NewV1(mockClient)
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
		name          string
		request       *systemapi.DeleteSystemRequest
		mockResponse  *systemgrpc.DeleteSystemResponse
		mockError     error
		expectedError error
		expectSuccess bool
	}{
		{
			name: "successful deletion",
			request: &systemapi.DeleteSystemRequest{
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
			expectedError: apierrors.NewValidationError("request", "request cannot be nil"),
		},
		{
			name: "missing region",
			request: &systemapi.DeleteSystemRequest{
				ExternalID: "ext-123",
			},
			expectedError: apierrors.NewValidationError("Region", "region is required"),
		},
		{
			name: "missing external ID",
			request: &systemapi.DeleteSystemRequest{
				Region: "us-east-1",
			},
			expectedError: apierrors.NewValidationError("ExternalID", "external_id is required"),
		},
		{
			name: "system not found",
			request: &systemapi.DeleteSystemRequest{
				Region:     "us-east-1",
				ExternalID: "non-existent",
			},
			mockError:     status.Error(codes.NotFound, "system not found"),
			expectedError: apierrors.ErrSystemNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockSystemClient{
				deleteSystemFunc: func(ctx context.Context, req *systemgrpc.DeleteSystemRequest) (*systemgrpc.DeleteSystemResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := system.NewV1(mockClient)
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

func TestV1_UpdateSystemStatus(t *testing.T) {
	tests := []struct {
		name          string
		request       *systemapi.UpdateSystemStatusRequest
		mockResponse  *systemgrpc.UpdateSystemStatusResponse
		mockError     error
		expectedError error
		expectSuccess bool
	}{
		{
			name: "successful update",
			request: &systemapi.UpdateSystemStatusRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				Type:       systemapi.SystemTypeKeystore,
				Status:     "ACTIVE",
			},
			mockResponse:  &systemgrpc.UpdateSystemStatusResponse{Success: true},
			expectSuccess: true,
		},
		{
			name:          "nil request",
			request:       nil,
			expectedError: apierrors.NewValidationError("request", "request cannot be nil"),
		},
		{
			name: "missing region",
			request: &systemapi.UpdateSystemStatusRequest{
				ExternalID: "ext-123",
				Type:       systemapi.SystemTypeKeystore,
			},
			expectedError: apierrors.NewValidationError("Region", "region is required"),
		},
		{
			name: "missing external_id",
			request: &systemapi.UpdateSystemStatusRequest{
				Region: "us-east-1",
				Type:   systemapi.SystemTypeKeystore,
			},
			expectedError: apierrors.NewValidationError("ExternalID", "external_id is required"),
		},
		{
			name: "unspecified type",
			request: &systemapi.UpdateSystemStatusRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				Type:       systemapi.SystemTypeUnspecified,
			},
			expectedError: apierrors.NewValidationError("Type", "type must be specified"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockSystemClient{
				updateSystemStatusFunc: func(ctx context.Context, req *systemgrpc.UpdateSystemStatusRequest) (*systemgrpc.UpdateSystemStatusResponse, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}
					return tt.mockResponse, nil
				},
			}

			v1 := system.NewV1(mockClient)
			resp, err := v1.UpdateSystemStatus(context.Background(), tt.request)

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

func TestV1_SetSystemLabels(t *testing.T) {
	tests := []struct {
		name          string
		request       *systemapi.SetSystemLabelsRequest
		mockResponse  *systemgrpc.SetSystemLabelsResponse
		mockError     error
		expectedError error
		expectSuccess bool
	}{
		{
			name: "successful set labels",
			request: &systemapi.SetSystemLabelsRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				Type:       systemapi.SystemTypeKeystore,
				Labels:     map[string]string{"env": "prod", "team": "platform"},
			},
			mockResponse:  &systemgrpc.SetSystemLabelsResponse{Success: true},
			expectSuccess: true,
		},
		{
			name:          "nil request",
			request:       nil,
			expectedError: apierrors.NewValidationError("request", "request cannot be nil"),
		},
		{
			name: "missing region",
			request: &systemapi.SetSystemLabelsRequest{
				ExternalID: "ext-123",
				Type:       systemapi.SystemTypeKeystore,
				Labels:     map[string]string{"env": "prod"},
			},
			expectedError: apierrors.NewValidationError("Region", "region is required"),
		},
		{
			name: "missing external_id",
			request: &systemapi.SetSystemLabelsRequest{
				Region: "us-east-1",
				Type:   systemapi.SystemTypeKeystore,
				Labels: map[string]string{"env": "prod"},
			},
			expectedError: apierrors.NewValidationError("ExternalID", "external_id is required"),
		},
		{
			name: "unspecified type",
			request: &systemapi.SetSystemLabelsRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				Type:       systemapi.SystemTypeUnspecified,
				Labels:     map[string]string{"env": "prod"},
			},
			expectedError: apierrors.NewValidationError("Type", "type must be specified"),
		},
		{
			name: "empty labels",
			request: &systemapi.SetSystemLabelsRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				Type:       systemapi.SystemTypeKeystore,
				Labels:     map[string]string{},
			},
			expectedError: apierrors.NewValidationError("Labels", "at least one label is required"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockSystemClient{
				setSystemLabelsFunc: func(ctx context.Context, req *systemgrpc.SetSystemLabelsRequest) (*systemgrpc.SetSystemLabelsResponse, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}
					return tt.mockResponse, nil
				},
			}

			v1 := system.NewV1(mockClient)
			resp, err := v1.SetSystemLabels(context.Background(), tt.request)

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

func TestV1_RemoveSystemLabels(t *testing.T) {
	tests := []struct {
		name          string
		request       *systemapi.RemoveSystemLabelsRequest
		mockResponse  *systemgrpc.RemoveSystemLabelsResponse
		mockError     error
		expectedError error
		expectSuccess bool
	}{
		{
			name: "successful remove labels",
			request: &systemapi.RemoveSystemLabelsRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				Type:       systemapi.SystemTypeKeystore,
				LabelKeys:  []string{"env", "team"},
			},
			mockResponse:  &systemgrpc.RemoveSystemLabelsResponse{Success: true},
			expectSuccess: true,
		},
		{
			name:          "nil request",
			request:       nil,
			expectedError: apierrors.NewValidationError("request", "request cannot be nil"),
		},
		{
			name: "missing region",
			request: &systemapi.RemoveSystemLabelsRequest{
				ExternalID: "ext-123",
				Type:       systemapi.SystemTypeKeystore,
				LabelKeys:  []string{"env"},
			},
			expectedError: apierrors.NewValidationError("Region", "region is required"),
		},
		{
			name: "missing external_id",
			request: &systemapi.RemoveSystemLabelsRequest{
				Region:    "us-east-1",
				Type:      systemapi.SystemTypeKeystore,
				LabelKeys: []string{"env"},
			},
			expectedError: apierrors.NewValidationError("ExternalID", "external_id is required"),
		},
		{
			name: "unspecified type",
			request: &systemapi.RemoveSystemLabelsRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				Type:       systemapi.SystemTypeUnspecified,
				LabelKeys:  []string{"env"},
			},
			expectedError: apierrors.NewValidationError("Type", "type must be specified"),
		},
		{
			name: "empty label keys",
			request: &systemapi.RemoveSystemLabelsRequest{
				Region:     "us-east-1",
				ExternalID: "ext-123",
				Type:       systemapi.SystemTypeKeystore,
				LabelKeys:  []string{},
			},
			expectedError: apierrors.NewValidationError("LabelKeys", "at least one label key is required"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockSystemClient{
				removeSystemLabelsFunc: func(ctx context.Context, req *systemgrpc.RemoveSystemLabelsRequest) (*systemgrpc.RemoveSystemLabelsResponse, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}
					return tt.mockResponse, nil
				},
			}

			v1 := system.NewV1(mockClient)
			resp, err := v1.RemoveSystemLabels(context.Background(), tt.request)

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
