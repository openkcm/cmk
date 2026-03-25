package system

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"

	"github.com/openkcm/cmk/internal/apiregistry/api/system"
)

type V1 struct {
	client systemgrpc.ServiceClient
}

var _ system.System = (*V1)(nil)

func NewV1(client systemgrpc.ServiceClient) *V1 {
	return &V1{
		client: client,
	}
}

func (v1 *V1) ListSystems(ctx context.Context, req *system.ListSystemsRequest) (*system.ListSystemsResponse, error) {
	if err := validateListSystemsRequest(req); err != nil {
		return nil, err
	}

	protoReq := &systemgrpc.ListSystemsRequest{
		Region:     req.Region,
		ExternalId: req.ExternalID,
		TenantId:   req.TenantID,
		Limit:      req.Limit,
		PageToken:  req.PageToken,
	}

	protoResp, err := v1.client.ListSystems(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	systems := make([]*system.SystemInfo, len(protoResp.GetSystems()))
	for i, protoSys := range protoResp.GetSystems() {
		systems[i] = mapProtoToSystemInfo(protoSys)
	}

	return &system.ListSystemsResponse{
		Systems:       systems,
		NextPageToken: protoResp.GetNextPageToken(),
	}, nil
}

func (v1 *V1) RegisterSystem(
	ctx context.Context, req *system.RegisterSystemRequest,
) (*system.RegisterSystemResponse, error) {
	if err := validateRegisterSystemRequest(req); err != nil {
		return nil, err
	}

	protoReq := &systemgrpc.RegisterSystemRequest{
		Region:        req.Region,
		ExternalId:    req.ExternalID,
		Type:          mapSystemTypeToProto(req.Type),
		TenantId:      req.TenantID,
		L2KeyId:       req.L2KeyID,
		HasL1KeyClaim: req.HasL1KeyClaim,
	}

	_, err := v1.client.RegisterSystem(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	return &system.RegisterSystemResponse{}, nil
}

func (v1 *V1) UpdateSystemL1KeyClaim(
	ctx context.Context, req *system.UpdateSystemL1KeyClaimRequest,
) (*system.UpdateSystemL1KeyClaimResponse, error) {
	if err := validateUpdateSystemL1KeyClaimRequest(req); err != nil {
		return nil, err
	}

	protoReq := &systemgrpc.UpdateSystemL1KeyClaimRequest{
		Region:     req.Region,
		ExternalId: req.ExternalID,
		TenantId:   req.TenantID,
		L1KeyClaim: req.L1KeyClaim,
	}

	protoResp, err := v1.client.UpdateSystemL1KeyClaim(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	return &system.UpdateSystemL1KeyClaimResponse{
		Success: protoResp.GetSuccess(),
	}, nil
}

func (v1 *V1) DeleteSystem(ctx context.Context, req *system.DeleteSystemRequest) (*system.DeleteSystemResponse, error) {
	if err := validateDeleteSystemRequest(req); err != nil {
		return nil, err
	}

	protoReq := &systemgrpc.DeleteSystemRequest{
		Region:     req.Region,
		ExternalId: req.ExternalID,
	}

	protoResp, err := v1.client.DeleteSystem(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	return &system.DeleteSystemResponse{
		Success: protoResp.GetSuccess(),
	}, nil
}

func mapProtoToSystemInfo(proto *systemgrpc.System) *system.SystemInfo {
	if proto == nil {
		return nil
	}

	return &system.SystemInfo{
		Region:        proto.GetRegion(),
		ExternalID:    proto.GetExternalId(),
		Type:          mapProtoToSystemType(proto.GetType()),
		TenantID:      proto.GetTenantId(),
		L2KeyID:       proto.GetL2KeyId(),
		HasL1KeyClaim: proto.GetHasL1KeyClaim(),
	}
}

func mapProtoToSystemType(protoType string) system.SystemType {
	switch strings.ToUpper(protoType) {
	case "KEYSTORE":
		return system.SystemTypeKeystore
	case "APPLICATION":
		return system.SystemTypeApplication
	default:
		return system.SystemTypeUnspecified
	}
}

func mapSystemTypeToProto(sysType system.SystemType) string {
	switch sysType {
	case system.SystemTypeKeystore:
		return "KEYSTORE"
	case system.SystemTypeApplication:
		return "APPLICATION"
	default:
		return "UNSPECIFIED"
	}
}

//nolint:cyclop,err113 // error mapping requires multiple case statements and dynamic errors
func convertGRPCError(err error) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	msg := st.Message()

	switch st.Code() {
	case codes.NotFound:
		if strings.Contains(msg, "system") || strings.Contains(msg, "not found") {
			return system.ErrSystemNotFound
		}
		return fmt.Errorf("not found: %s", msg)

	case codes.AlreadyExists:
		return system.ErrSystemAlreadyExists

	case codes.FailedPrecondition:
		if strings.Contains(msg, "key claim is already active") {
			return system.ErrL1KeyClaimAlreadyActive
		}
		if strings.Contains(msg, "key claim is already inactive") {
			return system.ErrL1KeyClaimAlreadyInactive
		}
		if strings.Contains(msg, "not linked to the tenant") || strings.Contains(msg, "not linked to tenant") {
			return system.ErrSystemNotLinkedToTenant
		}
		return fmt.Errorf("failed precondition: %s", msg)

	case codes.InvalidArgument:
		if strings.Contains(msg, "region") {
			return system.ErrInvalidRegion
		}
		if strings.Contains(msg, "external") {
			return system.ErrInvalidExternalID
		}
		if strings.Contains(msg, "tenant") {
			return system.ErrInvalidTenantID
		}
		if strings.Contains(msg, "type") {
			return system.ErrInvalidSystemType
		}
		return fmt.Errorf("invalid argument: %s", msg)

	default:
		return fmt.Errorf("gRPC error (%s): %s", st.Code(), msg)
	}
}

func validateListSystemsRequest(req *system.ListSystemsRequest) error {
	if req == nil {
		return system.NewValidationError("request", "request cannot be nil")
	}

	if req.Limit < 0 {
		return system.ErrInvalidLimit
	}

	return nil
}

func validateRegisterSystemRequest(req *system.RegisterSystemRequest) error {
	if req == nil {
		return system.NewValidationError("request", "request cannot be nil")
	}

	if req.Region == "" {
		return system.NewValidationError("Region", "region is required")
	}

	if req.ExternalID == "" {
		return system.NewValidationError("ExternalID", "external_id is required")
	}

	if req.TenantID == "" {
		return system.NewValidationError("TenantID", "tenant_id is required")
	}

	if req.Type == "" || req.Type == system.SystemTypeUnspecified {
		return system.NewValidationError("Type", "type must be specified")
	}

	return nil
}

func validateUpdateSystemL1KeyClaimRequest(req *system.UpdateSystemL1KeyClaimRequest) error {
	if req == nil {
		return system.NewValidationError("request", "request cannot be nil")
	}

	if req.Region == "" {
		return system.NewValidationError("Region", "region is required")
	}

	if req.ExternalID == "" {
		return system.NewValidationError("ExternalID", "external_id is required")
	}

	if req.TenantID == "" {
		return system.NewValidationError("TenantID", "tenant_id is required")
	}

	return nil
}

func validateDeleteSystemRequest(req *system.DeleteSystemRequest) error {
	if req == nil {
		return system.NewValidationError("request", "request cannot be nil")
	}

	if req.Region == "" {
		return system.NewValidationError("Region", "region is required")
	}

	if req.ExternalID == "" {
		return system.NewValidationError("ExternalID", "external_id is required")
	}

	return nil
}
