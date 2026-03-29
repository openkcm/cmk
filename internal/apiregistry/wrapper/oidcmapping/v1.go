package oidcmapping

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	oidcmappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"

	"github.com/openkcm/cmk/internal/apiregistry/api/oidcmapping"
	apierrors "github.com/openkcm/cmk/internal/apiregistry/errors"
)

type V1 struct {
	client oidcmappinggrpc.ServiceClient
}

func NewV1(client oidcmappinggrpc.ServiceClient) *V1 {
	return &V1{
		client: client,
	}
}

func (v *V1) ApplyOIDCMapping(
	ctx context.Context, req *oidcmapping.ApplyOIDCMappingRequest,
) (*oidcmapping.ApplyOIDCMappingResponse, error) {
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

func (v *V1) RemoveOIDCMapping(
	ctx context.Context, req *oidcmapping.RemoveOIDCMappingRequest,
) (*oidcmapping.RemoveOIDCMappingResponse, error) {
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

func (v *V1) BlockOIDCMapping(
	ctx context.Context, req *oidcmapping.BlockOIDCMappingRequest,
) (*oidcmapping.BlockOIDCMappingResponse, error) {
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

func (v *V1) UnblockOIDCMapping(
	ctx context.Context, req *oidcmapping.UnblockOIDCMappingRequest,
) (*oidcmapping.UnblockOIDCMappingResponse, error) {
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

func validateRequest(req any) error {
	if req == nil {
		return apierrors.NewValidationError("request", "request cannot be nil")
	}
	return nil
}

func validateTenantID(tenantID string) error {
	if tenantID == "" {
		return apierrors.NewValidationError("TenantID", "tenant ID is required")
	}
	return nil
}

func validateRequestWithTenantID(req any, tenantID string) error {
	if err := validateRequest(req); err != nil {
		return err
	}
	return validateTenantID(tenantID)
}

func validateApplyOIDCMappingRequest(req *oidcmapping.ApplyOIDCMappingRequest) error {
	if err := validateRequestWithTenantID(req, req.TenantID); err != nil {
		return err
	}
	if req.Issuer == "" {
		return apierrors.NewValidationError("Issuer", "issuer is required")
	}
	return nil
}

func validateRemoveOIDCMappingRequest(req *oidcmapping.RemoveOIDCMappingRequest) error {
	return validateRequestWithTenantID(req, req.TenantID)
}

func validateBlockOIDCMappingRequest(req *oidcmapping.BlockOIDCMappingRequest) error {
	return validateRequestWithTenantID(req, req.TenantID)
}

func validateUnblockOIDCMappingRequest(req *oidcmapping.UnblockOIDCMappingRequest) error {
	return validateRequestWithTenantID(req, req.TenantID)
}

//nolint:cyclop // error mapping requires multiple case statements
func convertGRPCError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return apierrors.ErrOIDCOperationFailed
	}

	switch st.Code() {
	case codes.NotFound:
		return apierrors.ErrOIDCMappingNotFound
	case codes.AlreadyExists:
		return apierrors.ErrOIDCMappingAlreadyExists
	case codes.InvalidArgument:
		msg := st.Message()
		switch {
		case strings.Contains(msg, "tenant"):
			return apierrors.ErrOIDCInvalidTenantID
		case strings.Contains(msg, "issuer"):
			return apierrors.ErrInvalidIssuer
		case strings.Contains(msg, "jwks"):
			return apierrors.ErrInvalidJwksURI
		case strings.Contains(msg, "audience"):
			return apierrors.ErrInvalidAudiences
		case strings.Contains(msg, "client"):
			return apierrors.ErrInvalidClientID
		default:
			return apierrors.ErrOIDCOperationFailed
		}
	case codes.FailedPrecondition:
		msg := st.Message()
		switch {
		case strings.Contains(msg, "already blocked"):
			return apierrors.ErrOIDCMappingAlreadyBlocked
		case strings.Contains(msg, "not blocked"):
			return apierrors.ErrOIDCMappingNotBlocked
		default:
			return apierrors.ErrOIDCOperationFailed
		}
	default:
		return apierrors.ErrOIDCOperationFailed
	}
}
