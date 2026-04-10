package auth

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"

	"github.com/openkcm/cmk/internal/apiregistry/api/auth"
	apierrors "github.com/openkcm/cmk/internal/apiregistry/errors"
)

type V1 struct {
	client authgrpc.ServiceClient
}

func NewV1(client authgrpc.ServiceClient) *V1 {
	return &V1{
		client: client,
	}
}

func (v1 *V1) ApplyAuth(
	ctx context.Context, req *authapi.ApplyAuthRequest,
) (*authapi.ApplyAuthResponse, error) {
	if err := validateApplyAuthRequest(req); err != nil {
		return nil, err
	}

	protoReq := &authgrpc.ApplyAuthRequest{
		ExternalId: req.ExternalID,
		TenantId:   req.TenantID,
		Type:       req.Type,
		Properties: req.Properties,
	}

	protoResp, err := v1.client.ApplyAuth(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	return &authapi.ApplyAuthResponse{
		Success: protoResp.GetSuccess(),
	}, nil
}

func (v1 *V1) GetAuth(
	ctx context.Context, req *authapi.GetAuthRequest,
) (*authapi.GetAuthResponse, error) {
	if err := validateGetAuthRequest(req); err != nil {
		return nil, err
	}

	protoReq := &authgrpc.GetAuthRequest{
		ExternalId: req.ExternalID,
	}

	protoResp, err := v1.client.GetAuth(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	return &authapi.GetAuthResponse{
		Auth: mapProtoToAuthInfo(protoResp.GetAuth()),
	}, nil
}

func (v1 *V1) ListAuths(
	ctx context.Context, req *authapi.ListAuthsRequest,
) (*authapi.ListAuthsResponse, error) {
	if err := validateListAuthsRequest(req); err != nil {
		return nil, err
	}

	protoReq := &authgrpc.ListAuthsRequest{
		TenantId:      req.TenantID,
		Limit:         req.Limit,
		NextPageToken: req.NextPageToken,
	}

	protoResp, err := v1.client.ListAuths(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	auths := make([]*authapi.AuthInfo, len(protoResp.GetAuth()))
	for i, protoAuth := range protoResp.GetAuth() {
		auths[i] = mapProtoToAuthInfo(protoAuth)
	}

	return &authapi.ListAuthsResponse{
		Auths:         auths,
		NextPageToken: protoResp.GetNextPageToken(),
	}, nil
}

func (v1 *V1) RemoveAuth(
	ctx context.Context, req *authapi.RemoveAuthRequest,
) (*authapi.RemoveAuthResponse, error) {
	if err := validateRemoveAuthRequest(req); err != nil {
		return nil, err
	}

	protoReq := &authgrpc.RemoveAuthRequest{
		ExternalId: req.ExternalID,
	}

	protoResp, err := v1.client.RemoveAuth(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	return &authapi.RemoveAuthResponse{
		Success: protoResp.GetSuccess(),
	}, nil
}

func mapProtoToAuthInfo(proto *authgrpc.Auth) *authapi.AuthInfo {
	if proto == nil {
		return nil
	}

	return &authapi.AuthInfo{
		ExternalID:   proto.GetExternalId(),
		TenantID:     proto.GetTenantId(),
		Type:         proto.GetType(),
		Properties:   proto.GetProperties(),
		Status:       mapProtoToAuthStatus(proto.GetStatus()),
		ErrorMessage: proto.GetErrorMessage(),
		UpdatedAt:    proto.GetUpdatedAt(),
		CreatedAt:    proto.GetCreatedAt(),
	}
}

//nolint:cyclop // status mapping requires multiple case statements
func mapProtoToAuthStatus(protoStatus authgrpc.AuthStatus) authapi.AuthStatus {
	switch protoStatus {
	case authgrpc.AuthStatus_AUTH_STATUS_APPLYING:
		return authapi.AuthStatusApplying
	case authgrpc.AuthStatus_AUTH_STATUS_APPLYING_ERROR:
		return authapi.AuthStatusApplyingError
	case authgrpc.AuthStatus_AUTH_STATUS_APPLIED:
		return authapi.AuthStatusApplied
	case authgrpc.AuthStatus_AUTH_STATUS_REMOVING:
		return authapi.AuthStatusRemoving
	case authgrpc.AuthStatus_AUTH_STATUS_REMOVING_ERROR:
		return authapi.AuthStatusRemovingError
	case authgrpc.AuthStatus_AUTH_STATUS_REMOVED:
		return authapi.AuthStatusRemoved
	case authgrpc.AuthStatus_AUTH_STATUS_BLOCKING:
		return authapi.AuthStatusBlocking
	case authgrpc.AuthStatus_AUTH_STATUS_BLOCKING_ERROR:
		return authapi.AuthStatusBlockingError
	case authgrpc.AuthStatus_AUTH_STATUS_BLOCKED:
		return authapi.AuthStatusBlocked
	case authgrpc.AuthStatus_AUTH_STATUS_UNBLOCKING:
		return authapi.AuthStatusUnblocking
	case authgrpc.AuthStatus_AUTH_STATUS_UNBLOCKING_ERROR:
		return authapi.AuthStatusUnblockingError
	default:
		return authapi.AuthStatusUnspecified
	}
}

//nolint:cyclop // error mapping requires multiple case statements
func convertGRPCError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return apierrors.ErrAuthOperationFailed
	}

	switch st.Code() {
	case codes.NotFound:
		return apierrors.ErrAuthNotFound
	case codes.AlreadyExists:
		return apierrors.ErrAuthAlreadyExists
	case codes.InvalidArgument:
		msg := st.Message()
		switch {
		case strings.Contains(msg, "external"):
			return apierrors.ErrAuthInvalidExternalID
		case strings.Contains(msg, "tenant"):
			return apierrors.ErrAuthInvalidTenantID
		case strings.Contains(msg, "type"):
			return apierrors.ErrAuthInvalidType
		case strings.Contains(msg, "properties"):
			return apierrors.ErrAuthInvalidProperties
		default:
			return apierrors.ErrAuthOperationFailed
		}
	default:
		return apierrors.ErrAuthOperationFailed
	}
}

func validateExternalID(externalID string) error {
	if externalID == "" {
		return apierrors.NewValidationError("ExternalID", "external_id is required")
	}
	return nil
}

func validateTenantID(tenantID string) error {
	if tenantID == "" {
		return apierrors.NewValidationError("TenantID", "tenant_id is required")
	}
	return nil
}

func validateType(t string) error {
	if t == "" {
		return apierrors.NewValidationError("Type", "type is required")
	}
	return nil
}

func validateApplyAuthRequest(req *authapi.ApplyAuthRequest) error {
	if req == nil {
		return apierrors.NewValidationError("request", "request cannot be nil")
	}
	if err := validateExternalID(req.ExternalID); err != nil {
		return err
	}
	if err := validateTenantID(req.TenantID); err != nil {
		return err
	}
	if err := validateType(req.Type); err != nil {
		return err
	}
	if len(req.Properties) == 0 {
		return apierrors.NewValidationError("Properties", "at least one property is required")
	}
	return nil
}

func validateGetAuthRequest(req *authapi.GetAuthRequest) error {
	if req == nil {
		return apierrors.NewValidationError("request", "request cannot be nil")
	}
	return validateExternalID(req.ExternalID)
}

func validateListAuthsRequest(req *authapi.ListAuthsRequest) error {
	if req == nil {
		return apierrors.NewValidationError("request", "request cannot be nil")
	}
	if req.Limit < 0 || req.Limit > 1000 {
		return apierrors.ErrAuthInvalidLimit
	}
	return nil
}

func validateRemoveAuthRequest(req *authapi.RemoveAuthRequest) error {
	if req == nil {
		return apierrors.NewValidationError("request", "request cannot be nil")
	}
	return validateExternalID(req.ExternalID)
}
