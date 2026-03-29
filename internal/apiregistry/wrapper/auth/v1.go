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
	ctx context.Context, req *auth.ApplyAuthRequest,
) (*auth.ApplyAuthResponse, error) {
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

	return &auth.ApplyAuthResponse{
		Success: protoResp.GetSuccess(),
	}, nil
}

func (v1 *V1) GetAuth(
	ctx context.Context, req *auth.GetAuthRequest,
) (*auth.GetAuthResponse, error) {
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

	return &auth.GetAuthResponse{
		Auth: mapProtoToAuthInfo(protoResp.GetAuth()),
	}, nil
}

func (v1 *V1) ListAuths(
	ctx context.Context, req *auth.ListAuthsRequest,
) (*auth.ListAuthsResponse, error) {
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

	auths := make([]*auth.AuthInfo, len(protoResp.GetAuth()))
	for i, protoAuth := range protoResp.GetAuth() {
		auths[i] = mapProtoToAuthInfo(protoAuth)
	}

	return &auth.ListAuthsResponse{
		Auths:         auths,
		NextPageToken: protoResp.GetNextPageToken(),
	}, nil
}

func (v1 *V1) RemoveAuth(
	ctx context.Context, req *auth.RemoveAuthRequest,
) (*auth.RemoveAuthResponse, error) {
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

	return &auth.RemoveAuthResponse{
		Success: protoResp.GetSuccess(),
	}, nil
}

func mapProtoToAuthInfo(proto *authgrpc.Auth) *auth.AuthInfo {
	if proto == nil {
		return nil
	}

	return &auth.AuthInfo{
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
func mapProtoToAuthStatus(protoStatus authgrpc.AuthStatus) auth.AuthStatus {
	switch protoStatus {
	case authgrpc.AuthStatus_AUTH_STATUS_APPLYING:
		return auth.AuthStatusApplying
	case authgrpc.AuthStatus_AUTH_STATUS_APPLYING_ERROR:
		return auth.AuthStatusApplyingError
	case authgrpc.AuthStatus_AUTH_STATUS_APPLIED:
		return auth.AuthStatusApplied
	case authgrpc.AuthStatus_AUTH_STATUS_REMOVING:
		return auth.AuthStatusRemoving
	case authgrpc.AuthStatus_AUTH_STATUS_REMOVING_ERROR:
		return auth.AuthStatusRemovingError
	case authgrpc.AuthStatus_AUTH_STATUS_REMOVED:
		return auth.AuthStatusRemoved
	case authgrpc.AuthStatus_AUTH_STATUS_BLOCKING:
		return auth.AuthStatusBlocking
	case authgrpc.AuthStatus_AUTH_STATUS_BLOCKING_ERROR:
		return auth.AuthStatusBlockingError
	case authgrpc.AuthStatus_AUTH_STATUS_BLOCKED:
		return auth.AuthStatusBlocked
	case authgrpc.AuthStatus_AUTH_STATUS_UNBLOCKING:
		return auth.AuthStatusUnblocking
	case authgrpc.AuthStatus_AUTH_STATUS_UNBLOCKING_ERROR:
		return auth.AuthStatusUnblockingError
	default:
		return auth.AuthStatusUnspecified
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

func validateApplyAuthRequest(req *auth.ApplyAuthRequest) error {
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

func validateGetAuthRequest(req *auth.GetAuthRequest) error {
	if req == nil {
		return apierrors.NewValidationError("request", "request cannot be nil")
	}
	return validateExternalID(req.ExternalID)
}

func validateListAuthsRequest(req *auth.ListAuthsRequest) error {
	if req == nil {
		return apierrors.NewValidationError("request", "request cannot be nil")
	}
	if req.Limit < 0 || req.Limit > 1000 {
		return apierrors.ErrAuthInvalidLimit
	}
	return nil
}

func validateRemoveAuthRequest(req *auth.RemoveAuthRequest) error {
	if req == nil {
		return apierrors.NewValidationError("request", "request cannot be nil")
	}
	return validateExternalID(req.ExternalID)
}
