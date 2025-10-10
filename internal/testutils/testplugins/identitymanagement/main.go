package main

import (
	"context"

	"github.com/openkcm/plugin-sdk/pkg/plugin"

	idmangv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
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
	_ *idmangv1.GetUsersForGroupRequest,
) (*idmangv1.GetUsersForGroupResponse, error) {
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

func New() *TestPlugin {
	return &TestPlugin{}
}

func main() {
	server := New()

	plugin.Serve(idmangv1.IdentityManagementServicePluginServer(server))
}
