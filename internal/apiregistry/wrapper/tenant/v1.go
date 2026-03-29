package tenant

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk/internal/apiregistry/api/tenant"
	apierrors "github.com/openkcm/cmk/internal/apiregistry/errors"
)

type V1 struct {
	client tenantgrpc.ServiceClient
}

func NewV1(client tenantgrpc.ServiceClient) *V1 {
	return &V1{
		client: client,
	}
}

func (v *V1) RegisterTenant(
	ctx context.Context, req *tenant.RegisterTenantRequest,
) (*tenant.RegisterTenantResponse, error) {
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
	if err := validateRequestWithTenantID(req, req.ID); err != nil {
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
	if err := validateRequestWithTenantID(req, req.ID); err != nil {
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

func (v *V1) UnblockTenant(
	ctx context.Context, req *tenant.UnblockTenantRequest,
) (*tenant.UnblockTenantResponse, error) {
	if err := validateRequestWithTenantID(req, req.ID); err != nil {
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

func (v *V1) TerminateTenant(
	ctx context.Context, req *tenant.TerminateTenantRequest,
) (*tenant.TerminateTenantResponse, error) {
	if err := validateRequestWithTenantID(req, req.ID); err != nil {
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

func (v *V1) SetTenantLabels(
	ctx context.Context, req *tenant.SetTenantLabelsRequest,
) (*tenant.SetTenantLabelsResponse, error) {
	if err := validateRequestWithTenantID(req, req.ID); err != nil {
		return nil, err
	}
	if len(req.Labels) == 0 {
		return nil, apierrors.NewValidationError("Labels", "at least one label is required")
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

func (v *V1) RemoveTenantLabels(
	ctx context.Context, req *tenant.RemoveTenantLabelsRequest,
) (*tenant.RemoveTenantLabelsResponse, error) {
	if err := validateRequestWithTenantID(req, req.ID); err != nil {
		return nil, err
	}
	if len(req.LabelKeys) == 0 {
		return nil, apierrors.NewValidationError("LabelKeys", "at least one label key is required")
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

func (v *V1) SetTenantUserGroups(
	ctx context.Context, req *tenant.SetTenantUserGroupsRequest,
) (*tenant.SetTenantUserGroupsResponse, error) {
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

func validateRequest(req any) error {
	if req == nil {
		return apierrors.NewValidationError("request", "request cannot be nil")
	}
	return nil
}

func validateTenantID(id string) error {
	if id == "" {
		return apierrors.NewValidationError("ID", "tenant ID is required")
	}
	return nil
}

func validateRequestWithTenantID(req any, id string) error {
	if err := validateRequest(req); err != nil {
		return err
	}
	return validateTenantID(id)
}

func validateRegisterTenantRequest(req *tenant.RegisterTenantRequest) error {
	if err := validateRequest(req); err != nil {
		return err
	}
	if req.Region == "" {
		return apierrors.NewValidationError("Region", "region is required")
	}
	if req.OwnerID == "" {
		return apierrors.NewValidationError("OwnerID", "owner ID is required")
	}
	if req.OwnerType == "" {
		return apierrors.NewValidationError("OwnerType", "owner type is required")
	}
	return nil
}

func validateListTenantsRequest(req *tenant.ListTenantsRequest) error {
	if err := validateRequest(req); err != nil {
		return err
	}
	if req.Limit < 0 || req.Limit > 1000 {
		return apierrors.ErrInvalidLimit
	}
	return nil
}

func validateSetTenantUserGroupsRequest(req *tenant.SetTenantUserGroupsRequest) error {
	if err := validateRequestWithTenantID(req, req.ID); err != nil {
		return err
	}
	if len(req.UserGroups) == 0 {
		return apierrors.NewValidationError("UserGroups", "at least one user group is required")
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

//nolint:cyclop // status mapping requires multiple case statements
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

//nolint:cyclop // error mapping requires multiple case statements
func convertGRPCError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return apierrors.ErrTenantOperationFailed
	}

	switch st.Code() {
	case codes.NotFound:
		return apierrors.ErrTenantNotFound
	case codes.AlreadyExists:
		return apierrors.ErrTenantAlreadyExists
	case codes.InvalidArgument:
		return apierrors.ErrInvalidTenantID
	case codes.FailedPrecondition:
		msg := st.Message()
		switch {
		case strings.Contains(msg, "already blocked"):
			return apierrors.ErrTenantAlreadyBlocked
		case strings.Contains(msg, "not blocked"):
			return apierrors.ErrTenantNotBlocked
		case strings.Contains(msg, "already terminated"):
			return apierrors.ErrTenantAlreadyTerminated
		case strings.Contains(msg, "invalid status"):
			return apierrors.ErrInvalidTenantStatus
		default:
			return apierrors.ErrTenantOperationFailed
		}
	default:
		return apierrors.ErrTenantOperationFailed
	}
}
