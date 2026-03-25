package system

import "context"

type System interface {
	ListSystems(ctx context.Context, req *ListSystemsRequest) (*ListSystemsResponse, error)

	RegisterSystem(ctx context.Context, req *RegisterSystemRequest) (*RegisterSystemResponse, error)

	UpdateSystemL1KeyClaim(ctx context.Context, req *UpdateSystemL1KeyClaimRequest) (*UpdateSystemL1KeyClaimResponse, error)

	DeleteSystem(ctx context.Context, req *DeleteSystemRequest) (*DeleteSystemResponse, error)
}

type SystemType string

const (
	SystemTypeUnspecified SystemType = "UNSPECIFIED"
	SystemTypeKeystore    SystemType = "KEYSTORE"
	SystemTypeApplication SystemType = "APPLICATION"
)

type SystemInfo struct {
	Region        string
	ExternalID    string
	Type          SystemType
	TenantID      string
	L2KeyID       string
	HasL1KeyClaim bool
}

type ListSystemsRequest struct {
	Region     string
	ExternalID string
	TenantID   string
	Limit      int32
	PageToken  string
}

type ListSystemsResponse struct {
	Systems       []*SystemInfo
	NextPageToken string
}

type RegisterSystemRequest struct {
	Region        string
	ExternalID    string
	Type          SystemType
	TenantID      string
	L2KeyID       string
	HasL1KeyClaim bool
}

type RegisterSystemResponse struct {
}

type UpdateSystemL1KeyClaimRequest struct {
	Region     string
	ExternalID string
	TenantID   string
	L1KeyClaim bool
}

type UpdateSystemL1KeyClaimResponse struct {
	Success bool
}

type DeleteSystemRequest struct {
	Region     string
	ExternalID string
}

type DeleteSystemResponse struct {
	Success bool
}
