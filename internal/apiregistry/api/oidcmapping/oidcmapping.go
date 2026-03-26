package oidcmapping

type ApplyOIDCMappingRequest struct {
	TenantID   string
	Issuer     string
	JwksURI    *string
	Audiences  []string
	Properties map[string]string
	ClientID   *string
}

type ApplyOIDCMappingResponse struct {
	Success bool
	Message *string
}

type RemoveOIDCMappingRequest struct {
	TenantID string
}

type RemoveOIDCMappingResponse struct {
	Success bool
	Message *string
}

type BlockOIDCMappingRequest struct {
	TenantID string
}

type BlockOIDCMappingResponse struct {
	Success bool
	Message *string
}

type UnblockOIDCMappingRequest struct {
	TenantID string
}

type UnblockOIDCMappingResponse struct {
	Success bool
	Message *string
}
