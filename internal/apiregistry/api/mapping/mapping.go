package mapping

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
