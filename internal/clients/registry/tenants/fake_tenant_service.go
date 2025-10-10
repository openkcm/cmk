package tenants

import (
	"context"

	tenantv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
)

type FakeTenantService struct {
	tenantv1.UnimplementedServiceServer

	SetTenantUserGroupsError   error
	SetTenantUserGroupsSuccess bool
}

// NewFakeTenantService creates and returns a new instance of FakeTenantService
// with default values for SetTenantUserGroupsError = nil and SetTenantUserGroupsSuccess = true.
func NewFakeTenantService() *FakeTenantService {
	return &FakeTenantService{
		SetTenantUserGroupsSuccess: true,
	}
}

func (f *FakeTenantService) SetTenantUserGroups(
	_ context.Context,
	_ *tenantv1.SetTenantUserGroupsRequest,
) (*tenantv1.SetTenantUserGroupsResponse, error) {
	if f.SetTenantUserGroupsError != nil {
		return nil, f.SetTenantUserGroupsError
	}

	return &tenantv1.SetTenantUserGroupsResponse{
		Success: true,
	}, nil
}
