package mapping

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"

	"github.com/openkcm/cmk/internal/apiregistry/api/mapping"
	apierrors "github.com/openkcm/cmk/internal/apiregistry/errors"
)

type V1 struct {
	client mappinggrpc.ServiceClient
}

var _ mappingapi.RegistryMapping = (*V1)(nil)

func NewV1(client mappinggrpc.ServiceClient) *V1 {
	return &V1{
		client: client,
	}
}

func (v *V1) MapSystemToTenant(
	ctx context.Context, req *mappingapi.MapSystemToTenantRequest,
) (*mappingapi.MapSystemToTenantResponse, error) {
	if err := validateMapSystemToTenantRequest(req); err != nil {
		return nil, err
	}

	protoReq := &mappinggrpc.MapSystemToTenantRequest{
		ExternalId: req.ExternalID,
		Type:       req.Type,
		TenantId:   req.TenantID,
	}

	resp, err := v.client.MapSystemToTenant(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	return &mappingapi.MapSystemToTenantResponse{
		Success: resp.GetSuccess(),
	}, nil
}

func (v *V1) UnmapSystemFromTenant(
	ctx context.Context, req *mappingapi.UnmapSystemFromTenantRequest,
) (*mappingapi.UnmapSystemFromTenantResponse, error) {
	if err := validateUnmapSystemFromTenantRequest(req); err != nil {
		return nil, err
	}

	protoReq := &mappinggrpc.UnmapSystemFromTenantRequest{
		ExternalId: req.ExternalID,
		Type:       req.Type,
		TenantId:   req.TenantID,
	}

	resp, err := v.client.UnmapSystemFromTenant(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	return &mappingapi.UnmapSystemFromTenantResponse{
		Success: resp.GetSuccess(),
	}, nil
}

func (v *V1) Get(ctx context.Context, req *mappingapi.GetRequest) (*mappingapi.GetResponse, error) {
	if err := validateGetRequest(req); err != nil {
		return nil, err
	}

	protoReq := &mappinggrpc.GetRequest{
		ExternalId: req.ExternalID,
		Type:       req.Type,
	}

	resp, err := v.client.Get(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	return &mappingapi.GetResponse{
		TenantID: resp.GetTenantId(),
	}, nil
}

func validateRequest(req any) error {
	if req == nil {
		return apierrors.NewValidationError("request", "request cannot be nil")
	}
	return nil
}

func validateExternalID(externalID string) error {
	if externalID == "" {
		return apierrors.NewValidationError("ExternalID", "external ID is required")
	}
	return nil
}

func validateType(typeStr string) error {
	if typeStr == "" {
		return apierrors.NewValidationError("Type", "type is required")
	}
	return nil
}

func validateTenantID(tenantID string) error {
	if tenantID == "" {
		return apierrors.NewValidationError("TenantID", "tenant ID is required")
	}
	return nil
}

func validateExternalIDAndType(req any, externalID, typeStr string) error {
	if err := validateRequest(req); err != nil {
		return err
	}
	if err := validateExternalID(externalID); err != nil {
		return err
	}
	return validateType(typeStr)
}

func validateMapSystemToTenantRequest(req *mappingapi.MapSystemToTenantRequest) error {
	if err := validateExternalIDAndType(req, req.ExternalID, req.Type); err != nil {
		return err
	}
	return validateTenantID(req.TenantID)
}

func validateUnmapSystemFromTenantRequest(req *mappingapi.UnmapSystemFromTenantRequest) error {
	if err := validateExternalIDAndType(req, req.ExternalID, req.Type); err != nil {
		return err
	}
	return validateTenantID(req.TenantID)
}

func validateGetRequest(req *mappingapi.GetRequest) error {
	return validateExternalIDAndType(req, req.ExternalID, req.Type)
}

//nolint:cyclop // error mapping requires multiple case statements
func convertGRPCError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return apierrors.ErrMappingOperationFailed
	}

	switch st.Code() {
	case codes.NotFound:
		return apierrors.ErrMappingNotFound
	case codes.AlreadyExists:
		return apierrors.ErrMappingAlreadyExists
	case codes.InvalidArgument:
		// Try to determine the specific error from the message
		msg := st.Message()
		switch {
		case strings.Contains(msg, "external"):
			return apierrors.ErrMappingInvalidExternalID
		case strings.Contains(msg, "type"):
			return apierrors.ErrInvalidType
		case strings.Contains(msg, "tenant"):
			return apierrors.ErrMappingInvalidTenantID
		default:
			return apierrors.ErrMappingOperationFailed
		}
	case codes.FailedPrecondition:
		// System is not mapped to tenant
		return apierrors.ErrSystemNotMapped
	default:
		return apierrors.ErrMappingOperationFailed
	}
}
