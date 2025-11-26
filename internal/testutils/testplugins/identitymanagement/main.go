package main

import (
	"context"

	"github.com/openkcm/plugin-sdk/pkg/plugin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	idmangv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"

	"github.com/openkcm/cmk/internal/testutils"
)

type TestPlugin struct {
	configv1.UnsafeConfigServer
	idmangv1.UnsafeIdentityManagementServiceServer
}

var _ idmangv1.UnsafeIdentityManagementServiceServer = (*TestPlugin)(nil)

func (p *TestPlugin) Configure(_ context.Context, _ *configv1.ConfigureRequest) (*configv1.ConfigureResponse, error) {
	return &configv1.ConfigureResponse{}, nil
}

func (p *TestPlugin) GetUsersForGroup(
	_ context.Context,
	req *idmangv1.GetUsersForGroupRequest,
) (*idmangv1.GetUsersForGroupResponse, error) {
	users, ok := testutils.IdentityManagementGroupMembership[req.GetGroupId()]
	respUsers := make([]*idmangv1.User, 0, len(users))

	if ok {
		for _, u := range users {
			respUsers = append(respUsers, &idmangv1.User{
				Id:    u.ID,
				Email: u.Email,
			})
		}

		return &idmangv1.GetUsersForGroupResponse{
			Users: respUsers,
		}, nil
	}

	return &idmangv1.GetUsersForGroupResponse{}, nil
}

func (p *TestPlugin) GetGroupsForUser(
	_ context.Context,
	_ *idmangv1.GetGroupsForUserRequest,
) (*idmangv1.GetGroupsForUserResponse, error) {
	return &idmangv1.GetGroupsForUserResponse{}, nil
}

func (p *TestPlugin) GetAllGroups(
	_ context.Context,
	_ *idmangv1.GetAllGroupsRequest,
) (*idmangv1.GetAllGroupsResponse, error) {
	return &idmangv1.GetAllGroupsResponse{}, nil
}

func (p *TestPlugin) GetGroup(
	_ context.Context,
	req *idmangv1.GetGroupRequest,
) (*idmangv1.GetGroupResponse, error) {
	if g, ok := testutils.IdentityManagementGroups[req.GetGroupName()]; ok {
		return &idmangv1.GetGroupResponse{
			Group: &idmangv1.Group{
				Id:   g,
				Name: req.GetGroupName(),
			},
		}, nil
	}

	return nil, status.New(codes.NotFound, "group does not exist").Err()
}

func New() *TestPlugin {
	return &TestPlugin{}
}

func main() {
	server := New()

	plugin.Serve(idmangv1.IdentityManagementServicePluginServer(server))
}
