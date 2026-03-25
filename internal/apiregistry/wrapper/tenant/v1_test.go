package tenant

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk/internal/apiregistry/api/tenant"
)

// mockTenantClient is a mock implementation of tenantgrpc.ServiceClient
type mockTenantClient struct {
	registerTenantFunc       func(ctx context.Context, req *tenantgrpc.RegisterTenantRequest) (*tenantgrpc.RegisterTenantResponse, error)
	listTenantsFunc          func(ctx context.Context, req *tenantgrpc.ListTenantsRequest) (*tenantgrpc.ListTenantsResponse, error)
	getTenantFunc            func(ctx context.Context, req *tenantgrpc.GetTenantRequest) (*tenantgrpc.GetTenantResponse, error)
	blockTenantFunc          func(ctx context.Context, req *tenantgrpc.BlockTenantRequest) (*tenantgrpc.BlockTenantResponse, error)
	unblockTenantFunc        func(ctx context.Context, req *tenantgrpc.UnblockTenantRequest) (*tenantgrpc.UnblockTenantResponse, error)
	terminateTenantFunc      func(ctx context.Context, req *tenantgrpc.TerminateTenantRequest) (*tenantgrpc.TerminateTenantResponse, error)
	setTenantLabelsFunc      func(ctx context.Context, req *tenantgrpc.SetTenantLabelsRequest) (*tenantgrpc.SetTenantLabelsResponse, error)
	removeTenantLabelsFunc   func(ctx context.Context, req *tenantgrpc.RemoveTenantLabelsRequest) (*tenantgrpc.RemoveTenantLabelsResponse, error)
	setTenantUserGroupsFunc  func(ctx context.Context, req *tenantgrpc.SetTenantUserGroupsRequest) (*tenantgrpc.SetTenantUserGroupsResponse, error)
	tenantgrpc.ServiceClient // embed to satisfy interface
}

func (m *mockTenantClient) RegisterTenant(ctx context.Context, req *tenantgrpc.RegisterTenantRequest, opts ...grpc.CallOption) (*tenantgrpc.RegisterTenantResponse, error) {
	if m.registerTenantFunc != nil {
		return m.registerTenantFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockTenantClient) ListTenants(ctx context.Context, req *tenantgrpc.ListTenantsRequest, opts ...grpc.CallOption) (*tenantgrpc.ListTenantsResponse, error) {
	if m.listTenantsFunc != nil {
		return m.listTenantsFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockTenantClient) GetTenant(ctx context.Context, req *tenantgrpc.GetTenantRequest, opts ...grpc.CallOption) (*tenantgrpc.GetTenantResponse, error) {
	if m.getTenantFunc != nil {
		return m.getTenantFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockTenantClient) BlockTenant(ctx context.Context, req *tenantgrpc.BlockTenantRequest, opts ...grpc.CallOption) (*tenantgrpc.BlockTenantResponse, error) {
	if m.blockTenantFunc != nil {
		return m.blockTenantFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockTenantClient) UnblockTenant(ctx context.Context, req *tenantgrpc.UnblockTenantRequest, opts ...grpc.CallOption) (*tenantgrpc.UnblockTenantResponse, error) {
	if m.unblockTenantFunc != nil {
		return m.unblockTenantFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockTenantClient) TerminateTenant(ctx context.Context, req *tenantgrpc.TerminateTenantRequest, opts ...grpc.CallOption) (*tenantgrpc.TerminateTenantResponse, error) {
	if m.terminateTenantFunc != nil {
		return m.terminateTenantFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockTenantClient) SetTenantLabels(ctx context.Context, req *tenantgrpc.SetTenantLabelsRequest, opts ...grpc.CallOption) (*tenantgrpc.SetTenantLabelsResponse, error) {
	if m.setTenantLabelsFunc != nil {
		return m.setTenantLabelsFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockTenantClient) RemoveTenantLabels(ctx context.Context, req *tenantgrpc.RemoveTenantLabelsRequest, opts ...grpc.CallOption) (*tenantgrpc.RemoveTenantLabelsResponse, error) {
	if m.removeTenantLabelsFunc != nil {
		return m.removeTenantLabelsFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockTenantClient) SetTenantUserGroups(ctx context.Context, req *tenantgrpc.SetTenantUserGroupsRequest, opts ...grpc.CallOption) (*tenantgrpc.SetTenantUserGroupsResponse, error) {
	if m.setTenantUserGroupsFunc != nil {
		return m.setTenantUserGroupsFunc(ctx, req)
	}
	return nil, nil
}

func TestNewV1(t *testing.T) {
	mockClient := &mockTenantClient{}
	v1 := NewV1(mockClient)

	if v1 == nil {
		t.Fatal("expected non-nil V1 instance")
	}
	if v1.client == nil {
		t.Error("expected client to be set")
	}
}

func TestV1_RegisterTenant(t *testing.T) {
	tests := []struct {
		name           string
		request        *tenant.RegisterTenantRequest
		mockResponse   *tenantgrpc.RegisterTenantResponse
		mockError      error
		expectedError  error
		expectedID     string
	}{
		{
			name: "successful registration",
			request: &tenant.RegisterTenantRequest{
				Name:      "Test Tenant",
				ID:        "tenant-123",
				Region:    "us-east-1",
				OwnerID:   "owner-456",
				OwnerType: "user",
				Role:      tenant.TenantRoleLive,
				Labels:    map[string]string{"env": "prod"},
			},
			mockResponse: &tenantgrpc.RegisterTenantResponse{
				Id: "tenant-123",
			},
			expectedID: "tenant-123",
		},
		{
			name: "missing region",
			request: &tenant.RegisterTenantRequest{
				OwnerID:   "owner-456",
				OwnerType: "user",
			},
			expectedError: tenant.NewValidationError("Region", "region is required"),
		},
		{
			name: "missing owner ID",
			request: &tenant.RegisterTenantRequest{
				Region:    "us-east-1",
				OwnerType: "user",
			},
			expectedError: tenant.NewValidationError("OwnerID", "owner ID is required"),
		},
		{
			name: "missing owner type",
			request: &tenant.RegisterTenantRequest{
				Region:  "us-east-1",
				OwnerID: "owner-456",
			},
			expectedError: tenant.NewValidationError("OwnerType", "owner type is required"),
		},
		{
			name: "tenant already exists",
			request: &tenant.RegisterTenantRequest{
				Region:    "us-east-1",
				OwnerID:   "owner-456",
				OwnerType: "user",
			},
			mockError:     status.Error(codes.AlreadyExists, "tenant already exists"),
			expectedError: tenant.ErrTenantAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				registerTenantFunc: func(ctx context.Context, req *tenantgrpc.RegisterTenantRequest) (*tenantgrpc.RegisterTenantResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
			resp, err := v1.RegisterTenant(context.Background(), tt.request)

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

			if resp.ID != tt.expectedID {
				t.Errorf("expected ID %s, got %s", tt.expectedID, resp.ID)
			}
		})
	}
}

func TestV1_ListTenants(t *testing.T) {
	now := time.Now().Format(time.RFC3339)

	tests := []struct {
		name           string
		request        *tenant.ListTenantsRequest
		mockResponse   *tenantgrpc.ListTenantsResponse
		mockError      error
		expectedError  error
		expectedCount  int
	}{
		{
			name: "successful list",
			request: &tenant.ListTenantsRequest{
				Region: "us-east-1",
				Limit:  10,
			},
			mockResponse: &tenantgrpc.ListTenantsResponse{
				Tenants: []*tenantgrpc.Tenant{
					{
						Id:              "tenant-123",
						Name:            "Test Tenant",
						Region:          "us-east-1",
						OwnerId:         "owner-456",
						OwnerType:       "user",
						Status:          tenantgrpc.Status_STATUS_ACTIVE,
						StatusUpdatedAt: now,
						Role:            tenantgrpc.Role_ROLE_LIVE,
						UpdatedAt:       now,
						CreatedAt:       now,
					},
				},
				NextPageToken: "next-token",
			},
			expectedCount: 1,
		},
		{
			name: "invalid limit too low",
			request: &tenant.ListTenantsRequest{
				Limit: -1,
			},
			expectedError: tenant.ErrInvalidLimit,
		},
		{
			name: "invalid limit too high",
			request: &tenant.ListTenantsRequest{
				Limit: 1001,
			},
			expectedError: tenant.ErrInvalidLimit,
		},
		{
			name: "tenant not found",
			request: &tenant.ListTenantsRequest{
				ID: "non-existent",
			},
			mockError:     status.Error(codes.NotFound, "tenant not found"),
			expectedError: tenant.ErrTenantNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				listTenantsFunc: func(ctx context.Context, req *tenantgrpc.ListTenantsRequest) (*tenantgrpc.ListTenantsResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
			resp, err := v1.ListTenants(context.Background(), tt.request)

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

			if len(resp.Tenants) != tt.expectedCount {
				t.Errorf("expected %d tenants, got %d", tt.expectedCount, len(resp.Tenants))
			}
		})
	}
}

func TestV1_GetTenant(t *testing.T) {
	now := time.Now().Format(time.RFC3339)

	tests := []struct {
		name           string
		request        *tenant.GetTenantRequest
		mockResponse   *tenantgrpc.GetTenantResponse
		mockError      error
		expectedError  error
	}{
		{
			name: "successful get",
			request: &tenant.GetTenantRequest{
				ID: "tenant-123",
			},
			mockResponse: &tenantgrpc.GetTenantResponse{
				Tenant: &tenantgrpc.Tenant{
					Id:        "tenant-123",
					Name:      "Test Tenant",
					Region:    "us-east-1",
					OwnerId:   "owner-456",
					OwnerType: "user",
					Status:    tenantgrpc.Status_STATUS_ACTIVE,
					Role:      tenantgrpc.Role_ROLE_LIVE,
					CreatedAt: now,
				},
			},
		},
		{
			name: "missing ID",
			request: &tenant.GetTenantRequest{
				ID: "",
			},
			expectedError: tenant.NewValidationError("ID", "tenant ID is required"),
		},
		{
			name: "tenant not found",
			request: &tenant.GetTenantRequest{
				ID: "non-existent",
			},
			mockError:     status.Error(codes.NotFound, "tenant not found"),
			expectedError: tenant.ErrTenantNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				getTenantFunc: func(ctx context.Context, req *tenantgrpc.GetTenantRequest) (*tenantgrpc.GetTenantResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
			resp, err := v1.GetTenant(context.Background(), tt.request)

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

			if resp.Tenant == nil {
				t.Error("expected tenant to be set")
			}
		})
	}
}

func TestV1_BlockTenant(t *testing.T) {
	tests := []struct {
		name           string
		request        *tenant.BlockTenantRequest
		mockResponse   *tenantgrpc.BlockTenantResponse
		mockError      error
		expectedError  error
		expectSuccess  bool
	}{
		{
			name: "successful block",
			request: &tenant.BlockTenantRequest{
				ID: "tenant-123",
			},
			mockResponse: &tenantgrpc.BlockTenantResponse{
				Success: true,
			},
			expectSuccess: true,
		},
		{
			name: "missing ID",
			request: &tenant.BlockTenantRequest{
				ID: "",
			},
			expectedError: tenant.NewValidationError("ID", "tenant ID is required"),
		},
		{
			name: "tenant already blocked",
			request: &tenant.BlockTenantRequest{
				ID: "tenant-123",
			},
			mockError:     status.Error(codes.FailedPrecondition, "tenant is already blocked"),
			expectedError: tenant.ErrTenantAlreadyBlocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				blockTenantFunc: func(ctx context.Context, req *tenantgrpc.BlockTenantRequest) (*tenantgrpc.BlockTenantResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
			resp, err := v1.BlockTenant(context.Background(), tt.request)

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

func TestV1_UnblockTenant(t *testing.T) {
	tests := []struct {
		name           string
		request        *tenant.UnblockTenantRequest
		mockResponse   *tenantgrpc.UnblockTenantResponse
		mockError      error
		expectedError  error
		expectSuccess  bool
	}{
		{
			name: "successful unblock",
			request: &tenant.UnblockTenantRequest{
				ID: "tenant-123",
			},
			mockResponse: &tenantgrpc.UnblockTenantResponse{
				Success: true,
			},
			expectSuccess: true,
		},
		{
			name: "missing ID",
			request: &tenant.UnblockTenantRequest{
				ID: "",
			},
			expectedError: tenant.NewValidationError("ID", "tenant ID is required"),
		},
		{
			name: "tenant not blocked",
			request: &tenant.UnblockTenantRequest{
				ID: "tenant-123",
			},
			mockError:     status.Error(codes.FailedPrecondition, "tenant is not blocked"),
			expectedError: tenant.ErrTenantNotBlocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				unblockTenantFunc: func(ctx context.Context, req *tenantgrpc.UnblockTenantRequest) (*tenantgrpc.UnblockTenantResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
			resp, err := v1.UnblockTenant(context.Background(), tt.request)

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

func TestV1_TerminateTenant(t *testing.T) {
	tests := []struct {
		name           string
		request        *tenant.TerminateTenantRequest
		mockResponse   *tenantgrpc.TerminateTenantResponse
		mockError      error
		expectedError  error
		expectSuccess  bool
	}{
		{
			name: "successful termination",
			request: &tenant.TerminateTenantRequest{
				ID: "tenant-123",
			},
			mockResponse: &tenantgrpc.TerminateTenantResponse{
				Success: true,
			},
			expectSuccess: true,
		},
		{
			name: "missing ID",
			request: &tenant.TerminateTenantRequest{
				ID: "",
			},
			expectedError: tenant.NewValidationError("ID", "tenant ID is required"),
		},
		{
			name: "tenant already terminated",
			request: &tenant.TerminateTenantRequest{
				ID: "tenant-123",
			},
			mockError:     status.Error(codes.FailedPrecondition, "tenant is already terminated"),
			expectedError: tenant.ErrTenantAlreadyTerminated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				terminateTenantFunc: func(ctx context.Context, req *tenantgrpc.TerminateTenantRequest) (*tenantgrpc.TerminateTenantResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
			resp, err := v1.TerminateTenant(context.Background(), tt.request)

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

func TestV1_SetTenantLabels(t *testing.T) {
	tests := []struct {
		name           string
		request        *tenant.SetTenantLabelsRequest
		mockResponse   *tenantgrpc.SetTenantLabelsResponse
		mockError      error
		expectedError  error
		expectSuccess  bool
	}{
		{
			name: "successful set labels",
			request: &tenant.SetTenantLabelsRequest{
				ID:     "tenant-123",
				Labels: map[string]string{"env": "prod", "team": "platform"},
			},
			mockResponse: &tenantgrpc.SetTenantLabelsResponse{
				Success: true,
			},
			expectSuccess: true,
		},
		{
			name: "missing ID",
			request: &tenant.SetTenantLabelsRequest{
				ID:     "",
				Labels: map[string]string{"env": "prod"},
			},
			expectedError: tenant.NewValidationError("ID", "tenant ID is required"),
		},
		{
			name: "empty labels",
			request: &tenant.SetTenantLabelsRequest{
				ID:     "tenant-123",
				Labels: map[string]string{},
			},
			expectedError: tenant.NewValidationError("Labels", "at least one label is required"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				setTenantLabelsFunc: func(ctx context.Context, req *tenantgrpc.SetTenantLabelsRequest) (*tenantgrpc.SetTenantLabelsResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
			resp, err := v1.SetTenantLabels(context.Background(), tt.request)

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

func TestV1_RemoveTenantLabels(t *testing.T) {
	tests := []struct {
		name           string
		request        *tenant.RemoveTenantLabelsRequest
		mockResponse   *tenantgrpc.RemoveTenantLabelsResponse
		mockError      error
		expectedError  error
		expectSuccess  bool
	}{
		{
			name: "successful remove labels",
			request: &tenant.RemoveTenantLabelsRequest{
				ID:        "tenant-123",
				LabelKeys: []string{"env", "team"},
			},
			mockResponse: &tenantgrpc.RemoveTenantLabelsResponse{
				Success: true,
			},
			expectSuccess: true,
		},
		{
			name: "missing ID",
			request: &tenant.RemoveTenantLabelsRequest{
				ID:        "",
				LabelKeys: []string{"env"},
			},
			expectedError: tenant.NewValidationError("ID", "tenant ID is required"),
		},
		{
			name: "empty label keys",
			request: &tenant.RemoveTenantLabelsRequest{
				ID:        "tenant-123",
				LabelKeys: []string{},
			},
			expectedError: tenant.NewValidationError("LabelKeys", "at least one label key is required"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				removeTenantLabelsFunc: func(ctx context.Context, req *tenantgrpc.RemoveTenantLabelsRequest) (*tenantgrpc.RemoveTenantLabelsResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
			resp, err := v1.RemoveTenantLabels(context.Background(), tt.request)

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

func TestV1_SetTenantUserGroups(t *testing.T) {
	tests := []struct {
		name           string
		request        *tenant.SetTenantUserGroupsRequest
		mockResponse   *tenantgrpc.SetTenantUserGroupsResponse
		mockError      error
		expectedError  error
		expectSuccess  bool
	}{
		{
			name: "successful set user groups",
			request: &tenant.SetTenantUserGroupsRequest{
				ID:         "tenant-123",
				UserGroups: []string{"admin", "developer"},
			},
			mockResponse: &tenantgrpc.SetTenantUserGroupsResponse{
				Success: true,
			},
			expectSuccess: true,
		},
		{
			name: "missing ID",
			request: &tenant.SetTenantUserGroupsRequest{
				ID:         "",
				UserGroups: []string{"admin"},
			},
			expectedError: tenant.NewValidationError("ID", "tenant ID is required"),
		},
		{
			name: "empty user groups",
			request: &tenant.SetTenantUserGroupsRequest{
				ID:         "tenant-123",
				UserGroups: []string{},
			},
			expectedError: tenant.NewValidationError("UserGroups", "at least one user group is required"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				setTenantUserGroupsFunc: func(ctx context.Context, req *tenantgrpc.SetTenantUserGroupsRequest) (*tenantgrpc.SetTenantUserGroupsResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := NewV1(mockClient)
			resp, err := v1.SetTenantUserGroups(context.Background(), tt.request)

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

func TestMapProtoToTenantStatus(t *testing.T) {
	tests := []struct {
		name        string
		protoStatus tenantgrpc.Status
		expected    tenant.TenantStatus
	}{
		{
			name:        "active",
			protoStatus: tenantgrpc.Status_STATUS_ACTIVE,
			expected:    tenant.TenantStatusActive,
		},
		{
			name:        "requested",
			protoStatus: tenantgrpc.Status_STATUS_REQUESTED,
			expected:    tenant.TenantStatusRequested,
		},
		{
			name:        "blocked",
			protoStatus: tenantgrpc.Status_STATUS_BLOCKED,
			expected:    tenant.TenantStatusBlocked,
		},
		{
			name:        "terminated",
			protoStatus: tenantgrpc.Status_STATUS_TERMINATED,
			expected:    tenant.TenantStatusTerminated,
		},
		{
			name:        "unspecified",
			protoStatus: 999, // Invalid status
			expected:    tenant.TenantStatusUnspecified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapProtoToTenantStatus(tt.protoStatus)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMapProtoToTenantRole(t *testing.T) {
	tests := []struct {
		name      string
		protoRole tenantgrpc.Role
		expected  tenant.TenantRole
	}{
		{
			name:      "live",
			protoRole: tenantgrpc.Role_ROLE_LIVE,
			expected:  tenant.TenantRoleLive,
		},
		{
			name:      "test",
			protoRole: tenantgrpc.Role_ROLE_TEST,
			expected:  tenant.TenantRoleTest,
		},
		{
			name:      "trial",
			protoRole: tenantgrpc.Role_ROLE_TRIAL,
			expected:  tenant.TenantRoleTrial,
		},
		{
			name:      "unspecified",
			protoRole: tenantgrpc.Role_ROLE_UNSPECIFIED,
			expected:  tenant.TenantRoleUnspecified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapProtoToTenantRole(tt.protoRole)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMapTenantRoleToProto(t *testing.T) {
	tests := []struct {
		name     string
		role     tenant.TenantRole
		expected tenantgrpc.Role
	}{
		{
			name:     "live",
			role:     tenant.TenantRoleLive,
			expected: tenantgrpc.Role_ROLE_LIVE,
		},
		{
			name:     "test",
			role:     tenant.TenantRoleTest,
			expected: tenantgrpc.Role_ROLE_TEST,
		},
		{
			name:     "trial",
			role:     tenant.TenantRoleTrial,
			expected: tenantgrpc.Role_ROLE_TRIAL,
		},
		{
			name:     "unspecified",
			role:     tenant.TenantRoleUnspecified,
			expected: tenantgrpc.Role_ROLE_UNSPECIFIED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapTenantRoleToProto(tt.role)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name     string
		timeStr  string
		expected time.Time
	}{
		{
			name:     "valid RFC3339",
			timeStr:  "2026-01-15T10:30:00Z",
			expected: mustParseTime("2026-01-15T10:30:00Z"),
		},
		{
			name:     "empty string",
			timeStr:  "",
			expected: time.Time{},
		},
		{
			name:     "invalid format",
			timeStr:  "invalid",
			expected: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTime(tt.timeStr)
			if !result.Equal(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
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
			name:          "tenant not found",
			inputError:    status.Error(codes.NotFound, "tenant not found"),
			expectedError: tenant.ErrTenantNotFound,
		},
		{
			name:          "tenant already exists",
			inputError:    status.Error(codes.AlreadyExists, "tenant already exists"),
			expectedError: tenant.ErrTenantAlreadyExists,
		},
		{
			name:          "invalid tenant ID",
			inputError:    status.Error(codes.InvalidArgument, "invalid tenant ID"),
			expectedError: tenant.ErrInvalidTenantID,
		},
		{
			name:          "tenant already blocked",
			inputError:    status.Error(codes.FailedPrecondition, "tenant is already blocked"),
			expectedError: tenant.ErrTenantAlreadyBlocked,
		},
		{
			name:          "tenant not blocked",
			inputError:    status.Error(codes.FailedPrecondition, "tenant is not blocked"),
			expectedError: tenant.ErrTenantNotBlocked,
		},
		{
			name:          "tenant already terminated",
			inputError:    status.Error(codes.FailedPrecondition, "tenant is already terminated"),
			expectedError: tenant.ErrTenantAlreadyTerminated,
		},
		{
			name:          "invalid status",
			inputError:    status.Error(codes.FailedPrecondition, "invalid status transition"),
			expectedError: tenant.ErrInvalidTenantStatus,
		},
		{
			name:          "operation failed",
			inputError:    status.Error(codes.Internal, "internal error"),
			expectedError: tenant.ErrOperationFailed,
		},
		{
			name:          "non-grpc error",
			inputError:    errors.New("network error"),
			expectedError: tenant.ErrOperationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertGRPCError(tt.inputError)
			if result != tt.expectedError {
				t.Errorf("expected error %v, got %v", tt.expectedError, result)
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
			s:        "blocked",
			substr:   "blocked",
			expected: true,
		},
		{
			name:     "substring at beginning",
			s:        "already blocked",
			substr:   "already",
			expected: true,
		},
		{
			name:     "substring at end",
			s:        "not blocked",
			substr:   "blocked",
			expected: true,
		},
		{
			name:     "substring in middle",
			s:        "tenant is not blocked",
			substr:   "not",
			expected: true,
		},
		{
			name:     "no match",
			s:        "terminated",
			substr:   "blocked",
			expected: false,
		},
		{
			name:     "empty string",
			s:        "",
			substr:   "blocked",
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

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
