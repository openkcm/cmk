package identity_management

import (
	"context"
	"fmt"

	"buf.build/go/protovalidate"
	"github.com/openkcm/plugin-sdk/api"
	"github.com/openkcm/plugin-sdk/pkg/plugin"

	grpcidentitymanagementv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
)

type V1 struct {
	plugin.Facade
	grpcidentitymanagementv1.IdentityManagementServicePluginClient
}

func (v1 *V1) Version() uint {
	return 1
}

func (v1 *V1) ServiceInfo() api.Info {
	return v1.Info
}

func (v1 *V1) GetGroup(
	ctx context.Context,
	req *identitymanagement.GetGroupRequest,
) (*identitymanagement.GetGroupResponse, error) {
	in := &grpcidentitymanagementv1.GetGroupRequest{
		GroupName:   req.GroupName,
		AuthContext: AuthContextToGRPC(&req.AuthContext),
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf("failed validation: %w", err)
	}

	grpcResp, err := v1.IdentityManagementServicePluginClient.GetGroup(ctx, in)
	if err != nil {
		return nil, err
	}
	return &identitymanagement.GetGroupResponse{
		Group: FromGRPCGroup(grpcResp.GetGroup()),
	}, nil
}

func (v1 *V1) ListGroups(
	ctx context.Context,
	req *identitymanagement.ListGroupsRequest,
) (*identitymanagement.ListGroupsResponse, error) {
	in := &grpcidentitymanagementv1.GetAllGroupsRequest{
		AuthContext: AuthContextToGRPC(&req.AuthContext),
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf("failed validation: %w", err)
	}

	grpcResp, err := v1.GetAllGroups(ctx, in)
	if err != nil {
		return nil, err
	}
	return &identitymanagement.ListGroupsResponse{
		Groups: FromGRPCGroups(grpcResp.GetGroups()),
	}, nil
}

func (v1 *V1) ListGroupUsers(
	ctx context.Context,
	req *identitymanagement.ListGroupUsersRequest,
) (*identitymanagement.ListGroupUsersResponse, error) {
	in := &grpcidentitymanagementv1.GetUsersForGroupRequest{
		GroupId:     req.GroupID,
		AuthContext: AuthContextToGRPC(&req.AuthContext),
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf("failed validation: %w", err)
	}

	grpcResp, err := v1.GetUsersForGroup(ctx, in)
	if err != nil {
		return nil, err
	}
	return &identitymanagement.ListGroupUsersResponse{
		Users: FromGRPCUsers(grpcResp.GetUsers()),
	}, nil
}

func (v1 *V1) ListUserGroups(
	ctx context.Context,
	req *identitymanagement.ListUserGroupsRequest,
) (*identitymanagement.ListUserGroupsResponse, error) {
	in := &grpcidentitymanagementv1.GetGroupsForUserRequest{
		UserId:      req.UserID,
		AuthContext: AuthContextToGRPC(&req.AuthContext),
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf("failed validation: %w", err)
	}

	grpcResp, err := v1.GetGroupsForUser(ctx, in)
	if err != nil {
		return nil, err
	}
	return &identitymanagement.ListUserGroupsResponse{
		Groups: FromGRPCGroups(grpcResp.GetGroups()),
	}, nil
}

func AuthContextToGRPC(v *identitymanagement.AuthContext) *grpcidentitymanagementv1.AuthContext {
	if v == nil {
		return nil
	}
	return &grpcidentitymanagementv1.AuthContext{
		Data: v.Data,
	}
}

func FromGRPCGroup(v *grpcidentitymanagementv1.Group) identitymanagement.Group {
	if v == nil {
		return identitymanagement.Group{}
	}
	return identitymanagement.Group{
		ID:   v.GetId(),
		Name: v.GetName(),
	}
}

func FromGRPCGroups(groups []*grpcidentitymanagementv1.Group) []identitymanagement.Group {
	wrapperGroups := make([]identitymanagement.Group, 0, len(groups))
	for _, group := range groups {
		wrapperGroups = append(wrapperGroups, FromGRPCGroup(group))
	}
	return wrapperGroups
}

func FromGRPCUser(v *grpcidentitymanagementv1.User) identitymanagement.User {
	if v == nil {
		return identitymanagement.User{}
	}
	return identitymanagement.User{
		ID:    v.GetId(),
		Name:  v.GetName(),
		Email: v.GetEmail(),
	}
}

func FromGRPCUsers(users []*grpcidentitymanagementv1.User) []identitymanagement.User {
	wrapperUsers := make([]identitymanagement.User, 0, len(users))
	for _, user := range users {
		wrapperUsers = append(wrapperUsers, FromGRPCUser(user))
	}
	return wrapperUsers
}
