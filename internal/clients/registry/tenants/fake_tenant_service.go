package tenants

import (
	"context"
	"sync"

	tenantv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
)

type FakeTenantService struct {
	tenantv1.UnimplementedServiceServer

	SetTenantUserGroupsError   error
	SetTenantUserGroupsSuccess bool

	// Synchronization helpers for tests
	mu            sync.Mutex
	LastGroupsReq *tenantv1.SetTenantUserGroupsRequest
	GroupsSetCh   chan *tenantv1.SetTenantUserGroupsRequest
}

// NewFakeTenantService creates and returns a new instance of FakeTenantService
// with default values for SetTenantUserGroupsError = nil and SetTenantUserGroupsSuccess = true.
func NewFakeTenantService() *FakeTenantService {
	return &FakeTenantService{
		SetTenantUserGroupsSuccess: true,
		GroupsSetCh:                make(chan *tenantv1.SetTenantUserGroupsRequest, 1),
	}
}

func (f *FakeTenantService) SetTenantUserGroups(
	_ context.Context,
	req *tenantv1.SetTenantUserGroupsRequest,
) (*tenantv1.SetTenantUserGroupsResponse, error) {
	if f.SetTenantUserGroupsError != nil {
		return nil, f.SetTenantUserGroupsError
	}

	// store last request for assertions
	f.mu.Lock()
	f.LastGroupsReq = req
	f.mu.Unlock()

	// notify tests (non-blocking)
	select {
	case f.GroupsSetCh <- req:
	default:
	}

	return &tenantv1.SetTenantUserGroupsResponse{
		Success: true,
	}, nil
}
