package oidcmapping

import (
	"context"

	oidcmappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"
	"github.com/openkcm/cmk/internal/apiregistry/api/oidcmapping"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type V1 struct {
	client oidcmappinggrpc.ServiceClient
}

func NewV1(client oidcmappinggrpc.ServiceClient) *V1 {
	return &V1{
		client: client,
	}
}

func (v *V1) ApplyOIDCMapping(ctx context.Context, req *oidcmapping.ApplyOIDCMappingRequest) (*oidcmapping.ApplyOIDCMappingResponse, error) {
	if err := validateApplyOIDCMappingRequest(req); err != nil {
		return nil, err
	}

	protoReq := &oidcmappinggrpc.ApplyOIDCMappingRequest{
		TenantId:   req.TenantID,
		Issuer:     req.Issuer,
		Audiences:  req.Audiences,
		Properties: req.Properties,
	}
	if req.JwksURI != nil {
		protoReq.JwksUri = req.JwksURI
	}
	if req.ClientID != nil {
		protoReq.ClientId = req.ClientID
	}

	resp, err := v.client.ApplyOIDCMapping(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	result := &oidcmapping.ApplyOIDCMappingResponse{
		Success: resp.GetSuccess(),
	}
	if msg := resp.GetMessage(); msg != "" {
		result.Message = &msg
	}

	return result, nil
}

func (v *V1) RemoveOIDCMapping(ctx context.Context, req *oidcmapping.RemoveOIDCMappingRequest) (*oidcmapping.RemoveOIDCMappingResponse, error) {
	if err := validateRemoveOIDCMappingRequest(req); err != nil {
		return nil, err
	}

	protoReq := &oidcmappinggrpc.RemoveOIDCMappingRequest{
		TenantId: req.TenantID,
	}

	resp, err := v.client.RemoveOIDCMapping(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	result := &oidcmapping.RemoveOIDCMappingResponse{
		Success: resp.GetSuccess(),
	}
	if msg := resp.GetMessage(); msg != "" {
		result.Message = &msg
	}

	return result, nil
}

func (v *V1) BlockOIDCMapping(ctx context.Context, req *oidcmapping.BlockOIDCMappingRequest) (*oidcmapping.BlockOIDCMappingResponse, error) {
	if err := validateBlockOIDCMappingRequest(req); err != nil {
		return nil, err
	}

	protoReq := &oidcmappinggrpc.BlockOIDCMappingRequest{
		TenantId: req.TenantID,
	}

	resp, err := v.client.BlockOIDCMapping(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	result := &oidcmapping.BlockOIDCMappingResponse{
		Success: resp.GetSuccess(),
	}
	if msg := resp.GetMessage(); msg != "" {
		result.Message = &msg
	}

	return result, nil
}

func (v *V1) UnblockOIDCMapping(ctx context.Context, req *oidcmapping.UnblockOIDCMappingRequest) (*oidcmapping.UnblockOIDCMappingResponse, error) {
	if err := validateUnblockOIDCMappingRequest(req); err != nil {
		return nil, err
	}

	protoReq := &oidcmappinggrpc.UnblockOIDCMappingRequest{
		TenantId: req.TenantID,
	}

	resp, err := v.client.UnblockOIDCMapping(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	result := &oidcmapping.UnblockOIDCMappingResponse{
		Success: resp.GetSuccess(),
	}
	if msg := resp.GetMessage(); msg != "" {
		result.Message = &msg
	}

	return result, nil
}

func validateApplyOIDCMappingRequest(req *oidcmapping.ApplyOIDCMappingRequest) error {
	if req.TenantID == "" {
		return oidcmapping.NewValidationError("TenantID", "tenant ID is required")
	}
	if req.Issuer == "" {
		return oidcmapping.NewValidationError("Issuer", "issuer is required")
	}
	return nil
}

func validateRemoveOIDCMappingRequest(req *oidcmapping.RemoveOIDCMappingRequest) error {
	if req.TenantID == "" {
		return oidcmapping.NewValidationError("TenantID", "tenant ID is required")
	}
	return nil
}

func validateBlockOIDCMappingRequest(req *oidcmapping.BlockOIDCMappingRequest) error {
	if req.TenantID == "" {
		return oidcmapping.NewValidationError("TenantID", "tenant ID is required")
	}
	return nil
}

func validateUnblockOIDCMappingRequest(req *oidcmapping.UnblockOIDCMappingRequest) error {
	if req.TenantID == "" {
		return oidcmapping.NewValidationError("TenantID", "tenant ID is required")
	}
	return nil
}

func convertGRPCError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return oidcmapping.ErrOperationFailed
	}

	switch st.Code() {
	case codes.NotFound:
		return oidcmapping.ErrOIDCMappingNotFound
	case codes.AlreadyExists:
		return oidcmapping.ErrOIDCMappingAlreadyExists
	case codes.InvalidArgument:
		msg := st.Message()
		switch {
		case contains(msg, "tenant"):
			return oidcmapping.ErrInvalidTenantID
		case contains(msg, "issuer"):
			return oidcmapping.ErrInvalidIssuer
		case contains(msg, "jwks"):
			return oidcmapping.ErrInvalidJwksURI
		case contains(msg, "audience"):
			return oidcmapping.ErrInvalidAudiences
		case contains(msg, "client"):
			return oidcmapping.ErrInvalidClientID
		default:
			return oidcmapping.ErrOperationFailed
		}
	case codes.FailedPrecondition:
		msg := st.Message()
		switch {
		case contains(msg, "already blocked"):
			return oidcmapping.ErrOIDCMappingAlreadyBlocked
		case contains(msg, "not blocked"):
			return oidcmapping.ErrOIDCMappingNotBlocked
		default:
			return oidcmapping.ErrOperationFailed
		}
	default:
		return oidcmapping.ErrOperationFailed
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
