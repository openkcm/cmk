package tenant

import (
	"context"
	"strings"
	"time"

	"github.com/samber/oops"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	tenantapi "github.com/openkcm/cmk/internal/apiregistry/api/tenant"
	apierrors "github.com/openkcm/cmk/internal/apiregistry/errors"
)

type V1 struct {
	client tenantgrpc.ServiceClient
}

var _ tenantapi.RegistryTenant = (*V1)(nil)

func NewV1(client tenantgrpc.ServiceClient) *V1 {
	return &V1{
		client: client,
	}
}

func (v *V1) RegisterTenant(
	ctx context.Context, req *tenantapi.RegisterTenantRequest,
) (*tenantapi.RegisterTenantResponse, error) {
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

	return &tenantapi.RegisterTenantResponse{
		ID: resp.GetId(),
	}, nil
}

func (v *V1) ListTenants(
	ctx context.Context,
	req *tenantapi.ListTenantsRequest) (*tenantapi.ListTenantsResponse, error) {
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

	tenants := make([]*tenantapi.TenantInfo, len(resp.GetTenants()))
	for i, t := range resp.GetTenants() {
		tenants[i] = mapProtoToTenantInfo(t)
	}

	return &tenantapi.ListTenantsResponse{
		Tenants:       tenants,
		NextPageToken: resp.GetNextPageToken(),
	}, nil
}

func (v *V1) GetTenant(ctx context.Context, req *tenantapi.GetTenantRequest) (*tenantapi.GetTenantResponse, error) {
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

	return &tenantapi.GetTenantResponse{
		Tenant: mapProtoToTenantInfo(resp.GetTenant()),
	}, nil
}

func (v *V1) BlockTenant(
	ctx context.Context,
	req *tenantapi.BlockTenantRequest) (*tenantapi.BlockTenantResponse, error) {
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

	return &tenantapi.BlockTenantResponse{
		Success: resp.GetSuccess(),
	}, nil
}

func (v *V1) UnblockTenant(
	ctx context.Context, req *tenantapi.UnblockTenantRequest,
) (*tenantapi.UnblockTenantResponse, error) {
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

	return &tenantapi.UnblockTenantResponse{
		Success: resp.GetSuccess(),
	}, nil
}

func (v *V1) TerminateTenant(
	ctx context.Context, req *tenantapi.TerminateTenantRequest,
) (*tenantapi.TerminateTenantResponse, error) {
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

	return &tenantapi.TerminateTenantResponse{
		Success: resp.GetSuccess(),
	}, nil
}

func (v *V1) SetTenantLabels(
	ctx context.Context, req *tenantapi.SetTenantLabelsRequest,
) (*tenantapi.SetTenantLabelsResponse, error) {
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

	return &tenantapi.SetTenantLabelsResponse{
		Success: resp.GetSuccess(),
	}, nil
}

func (v *V1) RemoveTenantLabels(
	ctx context.Context, req *tenantapi.RemoveTenantLabelsRequest,
) (*tenantapi.RemoveTenantLabelsResponse, error) {
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

	return &tenantapi.RemoveTenantLabelsResponse{
		Success: resp.GetSuccess(),
	}, nil
}

func (v *V1) SetTenantUserGroups(
	ctx context.Context, req *tenantapi.SetTenantUserGroupsRequest,
) (*tenantapi.SetTenantUserGroupsResponse, error) {
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

	return &tenantapi.SetTenantUserGroupsResponse{
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

func validateRegisterTenantRequest(req *tenantapi.RegisterTenantRequest) error {
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

func validateListTenantsRequest(req *tenantapi.ListTenantsRequest) error {
	if err := validateRequest(req); err != nil {
		return err
	}
	if req.Limit < 0 || req.Limit > 1000 {
		return apierrors.ErrInvalidLimit
	}
	return nil
}

func validateSetTenantUserGroupsRequest(req *tenantapi.SetTenantUserGroupsRequest) error {
	if err := validateRequestWithTenantID(req, req.ID); err != nil {
		return err
	}
	if len(req.UserGroups) == 0 {
		return apierrors.NewValidationError("UserGroups", "at least one user group is required")
	}
	return nil
}

func mapProtoToTenantInfo(protoTenant *tenantgrpc.Tenant) *tenantapi.TenantInfo {
	if protoTenant == nil {
		return nil
	}

	return &tenantapi.TenantInfo{
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
func mapProtoToTenantStatus(protoStatus tenantgrpc.Status) tenantapi.TenantStatus {
	switch protoStatus {
	case tenantgrpc.Status_STATUS_REQUESTED:
		return tenantapi.TenantStatusRequested
	case tenantgrpc.Status_STATUS_PROVISIONING:
		return tenantapi.TenantStatusProvisioning
	case tenantgrpc.Status_STATUS_PROVISIONING_ERROR:
		return tenantapi.TenantStatusProvisioningError
	case tenantgrpc.Status_STATUS_ACTIVE:
		return tenantapi.TenantStatusActive
	case tenantgrpc.Status_STATUS_BLOCKING:
		return tenantapi.TenantStatusBlocking
	case tenantgrpc.Status_STATUS_BLOCKING_ERROR:
		return tenantapi.TenantStatusBlockingError
	case tenantgrpc.Status_STATUS_BLOCKED:
		return tenantapi.TenantStatusBlocked
	case tenantgrpc.Status_STATUS_UNBLOCKING:
		return tenantapi.TenantStatusUnblocking
	case tenantgrpc.Status_STATUS_UNBLOCKING_ERROR:
		return tenantapi.TenantStatusUnblockingError
	case tenantgrpc.Status_STATUS_TERMINATING:
		return tenantapi.TenantStatusTerminating
	case tenantgrpc.Status_STATUS_TERMINATION_ERROR:
		return tenantapi.TenantStatusTerminationError
	case tenantgrpc.Status_STATUS_TERMINATED:
		return tenantapi.TenantStatusTerminated
	default:
		return tenantapi.TenantStatusUnspecified
	}
}

func mapProtoToTenantRole(protoRole tenantgrpc.Role) tenantapi.TenantRole {
	switch protoRole {
	case tenantgrpc.Role_ROLE_LIVE:
		return tenantapi.TenantRoleLive
	case tenantgrpc.Role_ROLE_TEST:
		return tenantapi.TenantRoleTest
	case tenantgrpc.Role_ROLE_TRIAL:
		return tenantapi.TenantRoleTrial
	default:
		return tenantapi.TenantRoleUnspecified
	}
}

func mapTenantRoleToProto(role tenantapi.TenantRole) tenantgrpc.Role {
	switch role {
	case tenantapi.TenantRoleLive:
		return tenantgrpc.Role_ROLE_LIVE
	case tenantapi.TenantRoleTest:
		return tenantgrpc.Role_ROLE_TEST
	case tenantapi.TenantRoleTrial:
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
		return oops.In("registry-tenant-grcp-client").Wrapf(err, "%w", apierrors.ErrTenantOperationFailed)
	}
}
