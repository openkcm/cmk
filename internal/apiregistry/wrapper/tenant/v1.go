package tenant

import (
	"context"
	"time"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	"github.com/openkcm/cmk/internal/apiregistry/api/tenant"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type V1 struct {
	client tenantgrpc.ServiceClient
}

func NewV1(client tenantgrpc.ServiceClient) *V1 {
	return &V1{
		client: client,
	}
}

func (v *V1) RegisterTenant(ctx context.Context, req *tenant.RegisterTenantRequest) (*tenant.RegisterTenantResponse, error) {
	if err := validateRegisterTenantRequest(req); err != nil {
		return nil, err
	}

	protoReq := &tenantgrpc.RegisterTenantRequest{
		Name:      req.Name,
		Id:        req.ID,
		Region:    req.Region,
		OwnerId:   req.OwnerID,
		OwnerType: req.OwnerType,
		Role:      mapTenantRoleToProto(req.Role),
		Labels:    req.Labels,
	}

	resp, err := v.client.RegisterTenant(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	return &tenant.RegisterTenantResponse{
		ID: resp.GetId(),
	}, nil
}

func (v *V1) ListTenants(ctx context.Context, req *tenant.ListTenantsRequest) (*tenant.ListTenantsResponse, error) {
	if err := validateListTenantsRequest(req); err != nil {
		return nil, err
	}

	protoReq := &tenantgrpc.ListTenantsRequest{
		Id:        req.ID,
		Name:      req.Name,
		Region:    req.Region,
		OwnerId:   req.OwnerID,
		OwnerType: req.OwnerType,
		Limit:     req.Limit,
		PageToken: req.PageToken,
		Labels:    req.Labels,
	}

	resp, err := v.client.ListTenants(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	tenants := make([]*tenant.TenantInfo, len(resp.GetTenants()))
	for i, t := range resp.GetTenants() {
		tenants[i] = mapProtoToTenantInfo(t)
	}

	return &tenant.ListTenantsResponse{
		Tenants:       tenants,
		NextPageToken: resp.GetNextPageToken(),
	}, nil
}

func (v *V1) GetTenant(ctx context.Context, req *tenant.GetTenantRequest) (*tenant.GetTenantResponse, error) {
	if err := validateGetTenantRequest(req); err != nil {
		return nil, err
	}

	protoReq := &tenantgrpc.GetTenantRequest{
		Id: req.ID,
	}

	resp, err := v.client.GetTenant(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	return &tenant.GetTenantResponse{
		Tenant: mapProtoToTenantInfo(resp.GetTenant()),
	}, nil
}

func (v *V1) BlockTenant(ctx context.Context, req *tenant.BlockTenantRequest) (*tenant.BlockTenantResponse, error) {
	if err := validateBlockTenantRequest(req); err != nil {
		return nil, err
	}

	protoReq := &tenantgrpc.BlockTenantRequest{
		Id: req.ID,
	}

	resp, err := v.client.BlockTenant(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	return &tenant.BlockTenantResponse{
		Success: resp.GetSuccess(),
	}, nil
}

func (v *V1) UnblockTenant(ctx context.Context, req *tenant.UnblockTenantRequest) (*tenant.UnblockTenantResponse, error) {
	if err := validateUnblockTenantRequest(req); err != nil {
		return nil, err
	}

	protoReq := &tenantgrpc.UnblockTenantRequest{
		Id: req.ID,
	}

	resp, err := v.client.UnblockTenant(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	return &tenant.UnblockTenantResponse{
		Success: resp.GetSuccess(),
	}, nil
}

func (v *V1) TerminateTenant(ctx context.Context, req *tenant.TerminateTenantRequest) (*tenant.TerminateTenantResponse, error) {
	if err := validateTerminateTenantRequest(req); err != nil {
		return nil, err
	}

	protoReq := &tenantgrpc.TerminateTenantRequest{
		Id: req.ID,
	}

	resp, err := v.client.TerminateTenant(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	return &tenant.TerminateTenantResponse{
		Success: resp.GetSuccess(),
	}, nil
}

func (v *V1) SetTenantLabels(ctx context.Context, req *tenant.SetTenantLabelsRequest) (*tenant.SetTenantLabelsResponse, error) {
	if err := validateSetTenantLabelsRequest(req); err != nil {
		return nil, err
	}

	protoReq := &tenantgrpc.SetTenantLabelsRequest{
		Id:     req.ID,
		Labels: req.Labels,
	}

	resp, err := v.client.SetTenantLabels(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	return &tenant.SetTenantLabelsResponse{
		Success: resp.GetSuccess(),
	}, nil
}

func (v *V1) RemoveTenantLabels(ctx context.Context, req *tenant.RemoveTenantLabelsRequest) (*tenant.RemoveTenantLabelsResponse, error) {
	if err := validateRemoveTenantLabelsRequest(req); err != nil {
		return nil, err
	}

	protoReq := &tenantgrpc.RemoveTenantLabelsRequest{
		Id:        req.ID,
		LabelKeys: req.LabelKeys,
	}

	resp, err := v.client.RemoveTenantLabels(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	return &tenant.RemoveTenantLabelsResponse{
		Success: resp.GetSuccess(),
	}, nil
}

func (v *V1) SetTenantUserGroups(ctx context.Context, req *tenant.SetTenantUserGroupsRequest) (*tenant.SetTenantUserGroupsResponse, error) {
	if err := validateSetTenantUserGroupsRequest(req); err != nil {
		return nil, err
	}

	protoReq := &tenantgrpc.SetTenantUserGroupsRequest{
		Id:         req.ID,
		UserGroups: req.UserGroups,
	}

	resp, err := v.client.SetTenantUserGroups(ctx, protoReq)
	if err != nil {
		return nil, convertGRPCError(err)
	}

	return &tenant.SetTenantUserGroupsResponse{
		Success: resp.GetSuccess(),
	}, nil
}

func validateRegisterTenantRequest(req *tenant.RegisterTenantRequest) error {
	if req.Region == "" {
		return tenant.NewValidationError("Region", "region is required")
	}
	if req.OwnerID == "" {
		return tenant.NewValidationError("OwnerID", "owner ID is required")
	}
	if req.OwnerType == "" {
		return tenant.NewValidationError("OwnerType", "owner type is required")
	}
	return nil
}

func validateListTenantsRequest(req *tenant.ListTenantsRequest) error {
	if req.Limit < 0 || req.Limit > 1000 {
		return tenant.ErrInvalidLimit
	}
	return nil
}

func validateGetTenantRequest(req *tenant.GetTenantRequest) error {
	if req.ID == "" {
		return tenant.NewValidationError("ID", "tenant ID is required")
	}
	return nil
}

func validateBlockTenantRequest(req *tenant.BlockTenantRequest) error {
	if req.ID == "" {
		return tenant.NewValidationError("ID", "tenant ID is required")
	}
	return nil
}

func validateUnblockTenantRequest(req *tenant.UnblockTenantRequest) error {
	if req.ID == "" {
		return tenant.NewValidationError("ID", "tenant ID is required")
	}
	return nil
}

func validateTerminateTenantRequest(req *tenant.TerminateTenantRequest) error {
	if req.ID == "" {
		return tenant.NewValidationError("ID", "tenant ID is required")
	}
	return nil
}

func validateSetTenantLabelsRequest(req *tenant.SetTenantLabelsRequest) error {
	if req.ID == "" {
		return tenant.NewValidationError("ID", "tenant ID is required")
	}
	if len(req.Labels) == 0 {
		return tenant.NewValidationError("Labels", "at least one label is required")
	}
	return nil
}

func validateRemoveTenantLabelsRequest(req *tenant.RemoveTenantLabelsRequest) error {
	if req.ID == "" {
		return tenant.NewValidationError("ID", "tenant ID is required")
	}
	if len(req.LabelKeys) == 0 {
		return tenant.NewValidationError("LabelKeys", "at least one label key is required")
	}
	return nil
}

func validateSetTenantUserGroupsRequest(req *tenant.SetTenantUserGroupsRequest) error {
	if req.ID == "" {
		return tenant.NewValidationError("ID", "tenant ID is required")
	}
	if len(req.UserGroups) == 0 {
		return tenant.NewValidationError("UserGroups", "at least one user group is required")
	}
	return nil
}

func mapProtoToTenantInfo(protoTenant *tenantgrpc.Tenant) *tenant.TenantInfo {
	if protoTenant == nil {
		return nil
	}

	return &tenant.TenantInfo{
		ID:              protoTenant.GetId(),
		Name:            protoTenant.GetName(),
		Region:          protoTenant.GetRegion(),
		OwnerID:         protoTenant.GetOwnerId(),
		OwnerType:       protoTenant.GetOwnerType(),
		Status:          mapProtoToTenantStatus(protoTenant.GetStatus()),
		StatusUpdatedAt: parseTime(protoTenant.GetStatusUpdatedAt()),
		Role:            mapProtoToTenantRole(protoTenant.GetRole()),
		UpdatedAt:       parseTime(protoTenant.GetUpdatedAt()),
		CreatedAt:       parseTime(protoTenant.GetCreatedAt()),
		Labels:          protoTenant.GetLabels(),
		UserGroups:      protoTenant.GetUserGroups(),
	}
}

func mapProtoToTenantStatus(protoStatus tenantgrpc.Status) tenant.TenantStatus {
	switch protoStatus {
	case tenantgrpc.Status_STATUS_REQUESTED:
		return tenant.TenantStatusRequested
	case tenantgrpc.Status_STATUS_PROVISIONING:
		return tenant.TenantStatusProvisioning
	case tenantgrpc.Status_STATUS_PROVISIONING_ERROR:
		return tenant.TenantStatusProvisioningError
	case tenantgrpc.Status_STATUS_ACTIVE:
		return tenant.TenantStatusActive
	case tenantgrpc.Status_STATUS_BLOCKING:
		return tenant.TenantStatusBlocking
	case tenantgrpc.Status_STATUS_BLOCKING_ERROR:
		return tenant.TenantStatusBlockingError
	case tenantgrpc.Status_STATUS_BLOCKED:
		return tenant.TenantStatusBlocked
	case tenantgrpc.Status_STATUS_UNBLOCKING:
		return tenant.TenantStatusUnblocking
	case tenantgrpc.Status_STATUS_UNBLOCKING_ERROR:
		return tenant.TenantStatusUnblockingError
	case tenantgrpc.Status_STATUS_TERMINATING:
		return tenant.TenantStatusTerminating
	case tenantgrpc.Status_STATUS_TERMINATION_ERROR:
		return tenant.TenantStatusTerminationError
	case tenantgrpc.Status_STATUS_TERMINATED:
		return tenant.TenantStatusTerminated
	default:
		return tenant.TenantStatusUnspecified
	}
}

func mapProtoToTenantRole(protoRole tenantgrpc.Role) tenant.TenantRole {
	switch protoRole {
	case tenantgrpc.Role_ROLE_LIVE:
		return tenant.TenantRoleLive
	case tenantgrpc.Role_ROLE_TEST:
		return tenant.TenantRoleTest
	case tenantgrpc.Role_ROLE_TRIAL:
		return tenant.TenantRoleTrial
	default:
		return tenant.TenantRoleUnspecified
	}
}

func mapTenantRoleToProto(role tenant.TenantRole) tenantgrpc.Role {
	switch role {
	case tenant.TenantRoleLive:
		return tenantgrpc.Role_ROLE_LIVE
	case tenant.TenantRoleTest:
		return tenantgrpc.Role_ROLE_TEST
	case tenant.TenantRoleTrial:
		return tenantgrpc.Role_ROLE_TRIAL
	default:
		return tenantgrpc.Role_ROLE_UNSPECIFIED
	}
}

func parseTime(timeStr string) time.Time {
	if timeStr == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Time{}
	}
	return t
}

func convertGRPCError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return tenant.ErrOperationFailed
	}

	switch st.Code() {
	case codes.NotFound:
		return tenant.ErrTenantNotFound
	case codes.AlreadyExists:
		return tenant.ErrTenantAlreadyExists
	case codes.InvalidArgument:
		return tenant.ErrInvalidTenantID
	case codes.FailedPrecondition:
		msg := st.Message()
		switch {
		case contains(msg, "already blocked"):
			return tenant.ErrTenantAlreadyBlocked
		case contains(msg, "not blocked"):
			return tenant.ErrTenantNotBlocked
		case contains(msg, "already terminated"):
			return tenant.ErrTenantAlreadyTerminated
		case contains(msg, "invalid status"):
			return tenant.ErrInvalidTenantStatus
		default:
			return tenant.ErrOperationFailed
		}
	default:
		return tenant.ErrOperationFailed
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
