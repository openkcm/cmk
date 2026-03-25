package mapping

import (
	"context"

	"github.com/openkcm/cmk/internal/apiregistry/service/api/mapping"
	mappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type V1 struct {
	client mappinggrpc.ServiceClient
}

func NewV1(client mappinggrpc.ServiceClient) *V1 {
	return &V1{
		client: client,
	}
}

func (v *V1) MapSystemToTenant(ctx context.Context, req *mapping.MapSystemToTenantRequest) (*mapping.MapSystemToTenantResponse, error) {
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

	return &mapping.MapSystemToTenantResponse{
		Success: resp.Success,
	}, nil
}

func (v *V1) UnmapSystemFromTenant(ctx context.Context, req *mapping.UnmapSystemFromTenantRequest) (*mapping.UnmapSystemFromTenantResponse, error) {
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

	return &mapping.UnmapSystemFromTenantResponse{
		Success: resp.Success,
	}, nil
}

func (v *V1) Get(ctx context.Context, req *mapping.GetRequest) (*mapping.GetResponse, error) {
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

	return &mapping.GetResponse{
		TenantID: resp.TenantId,
	}, nil
}

func validateMapSystemToTenantRequest(req *mapping.MapSystemToTenantRequest) error {
	if req.ExternalID == "" {
		return mapping.NewValidationError("ExternalID", "external ID is required")
	}
	if req.Type == "" {
		return mapping.NewValidationError("Type", "type is required")
	}
	if req.TenantID == "" {
		return mapping.NewValidationError("TenantID", "tenant ID is required")
	}
	return nil
}

func validateUnmapSystemFromTenantRequest(req *mapping.UnmapSystemFromTenantRequest) error {
	if req.ExternalID == "" {
		return mapping.NewValidationError("ExternalID", "external ID is required")
	}
	if req.Type == "" {
		return mapping.NewValidationError("Type", "type is required")
	}
	if req.TenantID == "" {
		return mapping.NewValidationError("TenantID", "tenant ID is required")
	}
	return nil
}

func validateGetRequest(req *mapping.GetRequest) error {
	if req.ExternalID == "" {
		return mapping.NewValidationError("ExternalID", "external ID is required")
	}
	if req.Type == "" {
		return mapping.NewValidationError("Type", "type is required")
	}
	return nil
}

func convertGRPCError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return mapping.ErrOperationFailed
	}

	switch st.Code() {
	case codes.NotFound:
		return mapping.ErrMappingNotFound
	case codes.AlreadyExists:
		return mapping.ErrMappingAlreadyExists
	case codes.InvalidArgument:
		// Try to determine the specific error from the message
		msg := st.Message()
		switch {
		case contains(msg, "external"):
			return mapping.ErrInvalidExternalID
		case contains(msg, "type"):
			return mapping.ErrInvalidType
		case contains(msg, "tenant"):
			return mapping.ErrInvalidTenantID
		default:
			return mapping.ErrOperationFailed
		}
	case codes.FailedPrecondition:
		// System is not mapped to tenant
		return mapping.ErrSystemNotMapped
	default:
		return mapping.ErrOperationFailed
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
