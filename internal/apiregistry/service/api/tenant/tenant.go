package tenant

import (
	"context"
	"time"
)

type TenantInfo struct {
	ID              string
	Name            string
	Region          string
	OwnerID         string
	OwnerType       string
	Status          TenantStatus
	StatusUpdatedAt time.Time
	Role            TenantRole
	UpdatedAt       time.Time
	CreatedAt       time.Time
	Labels          map[string]string
	UserGroups      []string
}

type TenantStatus int32

const (
	TenantStatusUnspecified       TenantStatus = 0
	TenantStatusRequested         TenantStatus = 1
	TenantStatusProvisioning      TenantStatus = 2
	TenantStatusProvisioningError TenantStatus = 3
	TenantStatusActive            TenantStatus = 4
	TenantStatusBlocking          TenantStatus = 5
	TenantStatusBlockingError     TenantStatus = 6
	TenantStatusBlocked           TenantStatus = 7
	TenantStatusUnblocking        TenantStatus = 8
	TenantStatusUnblockingError   TenantStatus = 9
	TenantStatusTerminating       TenantStatus = 10
	TenantStatusTerminationError  TenantStatus = 11
	TenantStatusTerminated        TenantStatus = 12
)

type TenantRole int32

const (
	TenantRoleUnspecified TenantRole = 0
	TenantRoleLive        TenantRole = 1
	TenantRoleTest        TenantRole = 2
	TenantRoleTrial       TenantRole = 3
)

type RegisterTenantRequest struct {
	Name      string
	ID        string
	Region    string
	OwnerID   string
	OwnerType string
	Role      TenantRole
	Labels    map[string]string
}

type RegisterTenantResponse struct {
	ID string
}

type ListTenantsRequest struct {
	ID        string
	Name      string
	Region    string
	OwnerID   string
	OwnerType string
	Limit     int32
	PageToken string
	Labels    map[string]string
}

type ListTenantsResponse struct {
	Tenants       []*TenantInfo
	NextPageToken string
}

type GetTenantRequest struct {
	ID string
}

type GetTenantResponse struct {
	Tenant *TenantInfo
}

type BlockTenantRequest struct {
	ID string
}

type BlockTenantResponse struct {
	Success bool
}

type UnblockTenantRequest struct {
	ID string
}

type UnblockTenantResponse struct {
	Success bool
}

type TerminateTenantRequest struct {
	ID string
}

type TerminateTenantResponse struct {
	Success bool
}

type SetTenantLabelsRequest struct {
	ID     string
	Labels map[string]string
}

type SetTenantLabelsResponse struct {
	Success bool
}

type RemoveTenantLabelsRequest struct {
	ID        string
	LabelKeys []string
}

type RemoveTenantLabelsResponse struct {
	Success bool
}

type SetTenantUserGroupsRequest struct {
	ID         string
	UserGroups []string
}

type SetTenantUserGroupsResponse struct {
	Success bool
}

type Tenant interface {
	RegisterTenant(ctx context.Context, req *RegisterTenantRequest) (*RegisterTenantResponse, error)
	ListTenants(ctx context.Context, req *ListTenantsRequest) (*ListTenantsResponse, error)
	GetTenant(ctx context.Context, req *GetTenantRequest) (*GetTenantResponse, error)
	BlockTenant(ctx context.Context, req *BlockTenantRequest) (*BlockTenantResponse, error)
	UnblockTenant(ctx context.Context, req *UnblockTenantRequest) (*UnblockTenantResponse, error)
	TerminateTenant(ctx context.Context, req *TerminateTenantRequest) (*TerminateTenantResponse, error)
	SetTenantLabels(ctx context.Context, req *SetTenantLabelsRequest) (*SetTenantLabelsResponse, error)
	RemoveTenantLabels(ctx context.Context, req *RemoveTenantLabelsRequest) (*RemoveTenantLabelsResponse, error)
	SetTenantUserGroups(ctx context.Context, req *SetTenantUserGroupsRequest) (*SetTenantUserGroupsResponse, error)
}
