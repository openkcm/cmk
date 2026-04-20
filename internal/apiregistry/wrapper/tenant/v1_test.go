package tenant_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	tenantapi "github.com/openkcm/cmk/internal/apiregistry/api/tenant"
	apierrors "github.com/openkcm/cmk/internal/apiregistry/errors"
	"github.com/openkcm/cmk/internal/apiregistry/wrapper/tenant"
)

// mockTenantClient is a mock implementation of tenantgrpc.ServiceClient
type mockTenantClient struct {
	tenantgrpc.ServiceClient // embed to satisfy interface

	registerTenantFunc      func(ctx context.Context, req *tenantgrpc.RegisterTenantRequest) (*tenantgrpc.RegisterTenantResponse, error)
	listTenantsFunc         func(ctx context.Context, req *tenantgrpc.ListTenantsRequest) (*tenantgrpc.ListTenantsResponse, error)
	getTenantFunc           func(ctx context.Context, req *tenantgrpc.GetTenantRequest) (*tenantgrpc.GetTenantResponse, error)
	blockTenantFunc         func(ctx context.Context, req *tenantgrpc.BlockTenantRequest) (*tenantgrpc.BlockTenantResponse, error)
	unblockTenantFunc       func(ctx context.Context, req *tenantgrpc.UnblockTenantRequest) (*tenantgrpc.UnblockTenantResponse, error)
	terminateTenantFunc     func(ctx context.Context, req *tenantgrpc.TerminateTenantRequest) (*tenantgrpc.TerminateTenantResponse, error)
	setTenantLabelsFunc     func(ctx context.Context, req *tenantgrpc.SetTenantLabelsRequest) (*tenantgrpc.SetTenantLabelsResponse, error)
	removeTenantLabelsFunc  func(ctx context.Context, req *tenantgrpc.RemoveTenantLabelsRequest) (*tenantgrpc.RemoveTenantLabelsResponse, error)
	setTenantUserGroupsFunc func(ctx context.Context, req *tenantgrpc.SetTenantUserGroupsRequest) (*tenantgrpc.SetTenantUserGroupsResponse, error)
}

func (m *mockTenantClient) RegisterTenant(ctx context.Context, req *tenantgrpc.RegisterTenantRequest, opts ...grpc.CallOption) (*tenantgrpc.RegisterTenantResponse, error) {
	if m.registerTenantFunc != nil {
		return m.registerTenantFunc(ctx, req)
	}
	return nil, nil //nolint:nilnil // mock implementation
}

func (m *mockTenantClient) ListTenants(ctx context.Context, req *tenantgrpc.ListTenantsRequest, opts ...grpc.CallOption) (*tenantgrpc.ListTenantsResponse, error) {
	if m.listTenantsFunc != nil {
		return m.listTenantsFunc(ctx, req)
	}
	return nil, nil //nolint:nilnil // mock implementation
}

func (m *mockTenantClient) GetTenant(ctx context.Context, req *tenantgrpc.GetTenantRequest, opts ...grpc.CallOption) (*tenantgrpc.GetTenantResponse, error) {
	if m.getTenantFunc != nil {
		return m.getTenantFunc(ctx, req)
	}
	return nil, nil //nolint:nilnil // mock implementation
}

func (m *mockTenantClient) BlockTenant(ctx context.Context, req *tenantgrpc.BlockTenantRequest, opts ...grpc.CallOption) (*tenantgrpc.BlockTenantResponse, error) {
	if m.blockTenantFunc != nil {
		return m.blockTenantFunc(ctx, req)
	}
	return nil, nil //nolint:nilnil // mock implementation
}

func (m *mockTenantClient) UnblockTenant(ctx context.Context, req *tenantgrpc.UnblockTenantRequest, opts ...grpc.CallOption) (*tenantgrpc.UnblockTenantResponse, error) {
	if m.unblockTenantFunc != nil {
		return m.unblockTenantFunc(ctx, req)
	}
	return nil, nil //nolint:nilnil // mock implementation
}

func (m *mockTenantClient) TerminateTenant(ctx context.Context, req *tenantgrpc.TerminateTenantRequest, opts ...grpc.CallOption) (*tenantgrpc.TerminateTenantResponse, error) {
	if m.terminateTenantFunc != nil {
		return m.terminateTenantFunc(ctx, req)
	}
	return nil, nil //nolint:nilnil // mock implementation
}

func (m *mockTenantClient) SetTenantLabels(ctx context.Context, req *tenantgrpc.SetTenantLabelsRequest, opts ...grpc.CallOption) (*tenantgrpc.SetTenantLabelsResponse, error) {
	if m.setTenantLabelsFunc != nil {
		return m.setTenantLabelsFunc(ctx, req)
	}
	return nil, nil //nolint:nilnil // mock implementation
}

func (m *mockTenantClient) RemoveTenantLabels(ctx context.Context, req *tenantgrpc.RemoveTenantLabelsRequest, opts ...grpc.CallOption) (*tenantgrpc.RemoveTenantLabelsResponse, error) {
	if m.removeTenantLabelsFunc != nil {
		return m.removeTenantLabelsFunc(ctx, req)
	}
	return nil, nil //nolint:nilnil // mock implementation
}

func (m *mockTenantClient) SetTenantUserGroups(ctx context.Context, req *tenantgrpc.SetTenantUserGroupsRequest, opts ...grpc.CallOption) (*tenantgrpc.SetTenantUserGroupsResponse, error) {
	if m.setTenantUserGroupsFunc != nil {
		return m.setTenantUserGroupsFunc(ctx, req)
	}
	return nil, nil //nolint:nilnil // mock implementation
}

func TestNewV1(t *testing.T) {
	mockClient := &mockTenantClient{}
	v1 := tenant.NewV1(mockClient)

	assert.NotNil(t, v1)
}

func TestV1_RegisterTenant(t *testing.T) {
	tests := []struct {
		name          string
		request       *tenantapi.RegisterTenantRequest
		mockResponse  *tenantgrpc.RegisterTenantResponse
		mockError     error
		expectedError error
		expectedID    string
	}{
		{
			name: "successful registration",
			request: &tenantapi.RegisterTenantRequest{
				Name:      "Test Tenant",
				ID:        "tenant-123",
				Region:    "us-east-1",
				OwnerID:   "owner-456",
				OwnerType: "user",
				Role:      tenantapi.TenantRoleLive,
				Labels:    map[string]string{"env": "prod"},
			},
			mockResponse: &tenantgrpc.RegisterTenantResponse{
				Id: "tenant-123",
			},
			expectedID: "tenant-123",
		},
		{
			name: "missing region",
			request: &tenantapi.RegisterTenantRequest{
				OwnerID:   "owner-456",
				OwnerType: "user",
			},
			expectedError: apierrors.NewValidationError("Region", "region is required"),
		},
		{
			name: "missing owner ID",
			request: &tenantapi.RegisterTenantRequest{
				Region:    "us-east-1",
				OwnerType: "user",
			},
			expectedError: apierrors.NewValidationError("OwnerID", "owner ID is required"),
		},
		{
			name: "missing owner type",
			request: &tenantapi.RegisterTenantRequest{
				Region:  "us-east-1",
				OwnerID: "owner-456",
			},
			expectedError: apierrors.NewValidationError("OwnerType", "owner type is required"),
		},
		{
			name: "tenant already exists",
			request: &tenantapi.RegisterTenantRequest{
				Region:    "us-east-1",
				OwnerID:   "owner-456",
				OwnerType: "user",
			},
			mockError:     status.Error(codes.AlreadyExists, "tenant already exists"),
			expectedError: apierrors.ErrTenantAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				registerTenantFunc: func(ctx context.Context, req *tenantgrpc.RegisterTenantRequest) (*tenantgrpc.RegisterTenantResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := tenant.NewV1(mockClient)
			resp, err := v1.RegisterTenant(context.Background(), tt.request)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedID, resp.ID)
		})
	}
}

func TestV1_ListTenants(t *testing.T) {
	now := time.Now().Format(time.RFC3339)

	tests := []struct {
		name          string
		request       *tenantapi.ListTenantsRequest
		mockResponse  *tenantgrpc.ListTenantsResponse
		mockError     error
		expectedError error
		expectedCount int
	}{
		{
			name: "successful list",
			request: &tenantapi.ListTenantsRequest{
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
			request: &tenantapi.ListTenantsRequest{
				Limit: -1,
			},
			expectedError: apierrors.ErrInvalidLimit,
		},
		{
			name: "invalid limit too high",
			request: &tenantapi.ListTenantsRequest{
				Limit: 1001,
			},
			expectedError: apierrors.ErrInvalidLimit,
		},
		{
			name: "tenant not found",
			request: &tenantapi.ListTenantsRequest{
				ID: "non-existent",
			},
			mockError:     status.Error(codes.NotFound, "tenant not found"),
			expectedError: apierrors.ErrTenantNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				listTenantsFunc: func(ctx context.Context, req *tenantgrpc.ListTenantsRequest) (*tenantgrpc.ListTenantsResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := tenant.NewV1(mockClient)
			resp, err := v1.ListTenants(context.Background(), tt.request)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				return
			}

			assert.NoError(t, err)
			assert.Len(t, resp.Tenants, tt.expectedCount)
		})
	}
}

func TestV1_GetTenant(t *testing.T) {
	now := time.Now().Format(time.RFC3339)

	tests := []struct {
		name          string
		request       *tenantapi.GetTenantRequest
		mockResponse  *tenantgrpc.GetTenantResponse
		mockError     error
		expectedError error
	}{
		{
			name: "successful get",
			request: &tenantapi.GetTenantRequest{
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
			request: &tenantapi.GetTenantRequest{
				ID: "",
			},
			expectedError: apierrors.NewValidationError("ID", "tenant ID is required"),
		},
		{
			name: "tenant not found",
			request: &tenantapi.GetTenantRequest{
				ID: "non-existent",
			},
			mockError:     status.Error(codes.NotFound, "tenant not found"),
			expectedError: apierrors.ErrTenantNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				getTenantFunc: func(ctx context.Context, req *tenantgrpc.GetTenantRequest) (*tenantgrpc.GetTenantResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := tenant.NewV1(mockClient)
			resp, err := v1.GetTenant(context.Background(), tt.request)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, resp.Tenant)
		})
	}
}

func TestV1_BlockTenant(t *testing.T) {
	tests := []struct {
		name          string
		request       *tenantapi.BlockTenantRequest
		mockResponse  *tenantgrpc.BlockTenantResponse
		mockError     error
		expectedError error
		expectSuccess bool
	}{
		{
			name: "successful block",
			request: &tenantapi.BlockTenantRequest{
				ID: "tenant-123",
			},
			mockResponse: &tenantgrpc.BlockTenantResponse{
				Success: true,
			},
			expectSuccess: true,
		},
		{
			name: "missing ID",
			request: &tenantapi.BlockTenantRequest{
				ID: "",
			},
			expectedError: apierrors.NewValidationError("ID", "tenant ID is required"),
		},
		{
			name: "tenant already blocked",
			request: &tenantapi.BlockTenantRequest{
				ID: "tenant-123",
			},
			mockError:     status.Error(codes.FailedPrecondition, "tenant is already blocked"),
			expectedError: apierrors.ErrTenantAlreadyBlocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				blockTenantFunc: func(ctx context.Context, req *tenantgrpc.BlockTenantRequest) (*tenantgrpc.BlockTenantResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := tenant.NewV1(mockClient)
			resp, err := v1.BlockTenant(context.Background(), tt.request)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectSuccess, resp.Success)
		})
	}
}

func TestV1_UnblockTenant(t *testing.T) {
	tests := []struct {
		name          string
		request       *tenantapi.UnblockTenantRequest
		mockResponse  *tenantgrpc.UnblockTenantResponse
		mockError     error
		expectedError error
		expectSuccess bool
	}{
		{
			name: "successful unblock",
			request: &tenantapi.UnblockTenantRequest{
				ID: "tenant-123",
			},
			mockResponse: &tenantgrpc.UnblockTenantResponse{
				Success: true,
			},
			expectSuccess: true,
		},
		{
			name: "missing ID",
			request: &tenantapi.UnblockTenantRequest{
				ID: "",
			},
			expectedError: apierrors.NewValidationError("ID", "tenant ID is required"),
		},
		{
			name: "tenant not blocked",
			request: &tenantapi.UnblockTenantRequest{
				ID: "tenant-123",
			},
			mockError:     status.Error(codes.FailedPrecondition, "tenant is not blocked"),
			expectedError: apierrors.ErrTenantNotBlocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				unblockTenantFunc: func(ctx context.Context, req *tenantgrpc.UnblockTenantRequest) (*tenantgrpc.UnblockTenantResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := tenant.NewV1(mockClient)
			resp, err := v1.UnblockTenant(context.Background(), tt.request)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectSuccess, resp.Success)
		})
	}
}

func TestV1_TerminateTenant(t *testing.T) {
	tests := []struct {
		name          string
		request       *tenantapi.TerminateTenantRequest
		mockResponse  *tenantgrpc.TerminateTenantResponse
		mockError     error
		expectedError error
		expectSuccess bool
	}{
		{
			name: "successful termination",
			request: &tenantapi.TerminateTenantRequest{
				ID: "tenant-123",
			},
			mockResponse: &tenantgrpc.TerminateTenantResponse{
				Success: true,
			},
			expectSuccess: true,
		},
		{
			name: "missing ID",
			request: &tenantapi.TerminateTenantRequest{
				ID: "",
			},
			expectedError: apierrors.NewValidationError("ID", "tenant ID is required"),
		},
		{
			name: "tenant already terminated",
			request: &tenantapi.TerminateTenantRequest{
				ID: "tenant-123",
			},
			mockError:     status.Error(codes.FailedPrecondition, "tenant is already terminated"),
			expectedError: apierrors.ErrTenantAlreadyTerminated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				terminateTenantFunc: func(ctx context.Context, req *tenantgrpc.TerminateTenantRequest) (*tenantgrpc.TerminateTenantResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := tenant.NewV1(mockClient)
			resp, err := v1.TerminateTenant(context.Background(), tt.request)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectSuccess, resp.Success)
		})
	}
}

func TestV1_SetTenantLabels(t *testing.T) {
	tests := []struct {
		name          string
		request       *tenantapi.SetTenantLabelsRequest
		mockResponse  *tenantgrpc.SetTenantLabelsResponse
		mockError     error
		expectedError error
		expectSuccess bool
	}{
		{
			name: "successful set labels",
			request: &tenantapi.SetTenantLabelsRequest{
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
			request: &tenantapi.SetTenantLabelsRequest{
				ID:     "",
				Labels: map[string]string{"env": "prod"},
			},
			expectedError: apierrors.NewValidationError("ID", "tenant ID is required"),
		},
		{
			name: "empty labels",
			request: &tenantapi.SetTenantLabelsRequest{
				ID:     "tenant-123",
				Labels: map[string]string{},
			},
			expectedError: apierrors.NewValidationError("Labels", "at least one label is required"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				setTenantLabelsFunc: func(ctx context.Context, req *tenantgrpc.SetTenantLabelsRequest) (*tenantgrpc.SetTenantLabelsResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := tenant.NewV1(mockClient)
			resp, err := v1.SetTenantLabels(context.Background(), tt.request)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectSuccess, resp.Success)
		})
	}
}

func TestV1_RemoveTenantLabels(t *testing.T) {
	tests := []struct {
		name          string
		request       *tenantapi.RemoveTenantLabelsRequest
		mockResponse  *tenantgrpc.RemoveTenantLabelsResponse
		mockError     error
		expectedError error
		expectSuccess bool
	}{
		{
			name: "successful remove labels",
			request: &tenantapi.RemoveTenantLabelsRequest{
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
			request: &tenantapi.RemoveTenantLabelsRequest{
				ID:        "",
				LabelKeys: []string{"env"},
			},
			expectedError: apierrors.NewValidationError("ID", "tenant ID is required"),
		},
		{
			name: "empty label keys",
			request: &tenantapi.RemoveTenantLabelsRequest{
				ID:        "tenant-123",
				LabelKeys: []string{},
			},
			expectedError: apierrors.NewValidationError("LabelKeys", "at least one label key is required"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				removeTenantLabelsFunc: func(ctx context.Context, req *tenantgrpc.RemoveTenantLabelsRequest) (*tenantgrpc.RemoveTenantLabelsResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := tenant.NewV1(mockClient)
			resp, err := v1.RemoveTenantLabels(context.Background(), tt.request)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectSuccess, resp.Success)
		})
	}
}

func TestV1_SetTenantUserGroups(t *testing.T) {
	tests := []struct {
		name          string
		request       *tenantapi.SetTenantUserGroupsRequest
		mockResponse  *tenantgrpc.SetTenantUserGroupsResponse
		mockError     error
		expectedError error
		expectSuccess bool
	}{
		{
			name: "successful set user groups",
			request: &tenantapi.SetTenantUserGroupsRequest{
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
			request: &tenantapi.SetTenantUserGroupsRequest{
				ID:         "",
				UserGroups: []string{"admin"},
			},
			expectedError: apierrors.NewValidationError("ID", "tenant ID is required"),
		},
		{
			name: "empty user groups",
			request: &tenantapi.SetTenantUserGroupsRequest{
				ID:         "tenant-123",
				UserGroups: []string{},
			},
			expectedError: apierrors.NewValidationError("UserGroups", "at least one user group is required"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockTenantClient{
				setTenantUserGroupsFunc: func(ctx context.Context, req *tenantgrpc.SetTenantUserGroupsRequest) (*tenantgrpc.SetTenantUserGroupsResponse, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			v1 := tenant.NewV1(mockClient)
			resp, err := v1.SetTenantUserGroups(context.Background(), tt.request)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectSuccess, resp.Success)
		})
	}
}
