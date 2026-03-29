package mappingapi

import "context"

type RegistryMapping interface {
	MapSystemToTenant(
		ctx context.Context, req *MapSystemToTenantRequest,
	) (*MapSystemToTenantResponse, error)
	UnmapSystemFromTenant(
		ctx context.Context, req *UnmapSystemFromTenantRequest,
	) (*UnmapSystemFromTenantResponse, error)
	Get(ctx context.Context, req *GetRequest) (*GetResponse, error)
}

type MapSystemToTenantRequest struct {
	ExternalID string
	Type       string
	TenantID   string
}

type MapSystemToTenantResponse struct {
	Success bool
}

type UnmapSystemFromTenantRequest struct {
	ExternalID string
	Type       string
	TenantID   string
}

type UnmapSystemFromTenantResponse struct {
	Success bool
}

type GetRequest struct {
	ExternalID string
	Type       string
}

type GetResponse struct {
	TenantID string
}
