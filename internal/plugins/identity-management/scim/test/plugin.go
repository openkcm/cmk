package testplugin

import (
	"context"

	"github.com/hashicorp/go-hclog"

	idmangv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
)

// TestPlugin is a simple test implementation of KeystoreProviderServer
type TestPlugin struct {
	idmangv1.UnsafeIdentityManagementServiceServer
	configv1.UnsafeConfigServer

	logger hclog.Logger
}

func NewTestPlugin() *TestPlugin {
	return &TestPlugin{}
}

func (p *TestPlugin) GetGroup(
	ctx context.Context,
	request *idmangv1.GetGroupRequest,
) (*idmangv1.GetGroupResponse, error) {
	return &idmangv1.GetGroupResponse{}, nil
}

func (p *TestPlugin) GetAllGroups(
	ctx context.Context,
	request *idmangv1.GetAllGroupsRequest,
) (*idmangv1.GetAllGroupsResponse, error) {
	p.logger.Info("GetAllGroups method has been called;")
	return &idmangv1.GetAllGroupsResponse{}, nil
}

func (p *TestPlugin) GetUsersForGroup(
	ctx context.Context,
	request *idmangv1.GetUsersForGroupRequest,
) (*idmangv1.GetUsersForGroupResponse, error) {
	p.logger.Info("GetUsersForGroup method has been called;")
	return &idmangv1.GetUsersForGroupResponse{}, nil
}

func (p *TestPlugin) GetGroupsForUser(
	ctx context.Context,
	request *idmangv1.GetGroupsForUserRequest,
) (*idmangv1.GetGroupsForUserResponse, error) {
	p.logger.Info("GetGroupsForUser method has been called;")
	return &idmangv1.GetGroupsForUserResponse{}, nil
}

func (p *TestPlugin) SetLogger(logger hclog.Logger) {
	p.logger = logger
	p.logger.Info("SetLogger method has been called;")
}

// Configure configures the plugin.
func (p *TestPlugin) Configure(
	ctx context.Context,
	req *configv1.ConfigureRequest,
) (*configv1.ConfigureResponse, error) {
	p.logger.Info("Configure method has been called;")
	return &configv1.ConfigureResponse{}, nil
}
