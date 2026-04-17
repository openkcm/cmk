package oidcmappingapi

import "context"

type SessionManagerOIDCMapping interface {
	ApplyOIDCMapping(ctx context.Context, req *ApplyOIDCMappingRequest) (*ApplyOIDCMappingResponse, error)
	RemoveOIDCMapping(ctx context.Context, req *RemoveOIDCMappingRequest) (*RemoveOIDCMappingResponse, error)
	BlockOIDCMapping(ctx context.Context, req *BlockOIDCMappingRequest) (*BlockOIDCMappingResponse, error)
	UnblockOIDCMapping(ctx context.Context, req *UnblockOIDCMappingRequest) (*UnblockOIDCMappingResponse, error)
}

type ApplyOIDCMappingRequest struct {
	TenantID   string
	Issuer     string
	JwksURI    *string
	Audiences  []string
	Properties map[string]string
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
