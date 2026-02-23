package noop

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/openkcm/plugin-sdk/pkg/hclog2slog"

	idmangv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
)

func Register(registry catalog.BuiltInPluginRegistry) {
	registry.Register(builtin(NewPlugin()))
}

func builtin(p *Plugin) catalog.BuiltInPlugin {
	return catalog.MakeBuiltIn("noop",
		idmangv1.IdentityManagementServicePluginServer(p),
		configv1.ConfigServiceServer(p))
}

// Plugin is a simple test implementation of KeystoreProviderServer
type Plugin struct {
	idmangv1.UnsafeIdentityManagementServiceServer
	configv1.UnsafeConfigServer

	logger    *slog.Logger
	buildInfo string
}

var (
	_ idmangv1.IdentityManagementServiceServer = (*Plugin)(nil)
	_ configv1.ConfigServer                    = (*Plugin)(nil)
)

func NewPlugin() *Plugin {
	return &Plugin{
		buildInfo: "{}",
	}
}

func (p *Plugin) SetLogger(logger hclog.Logger) {
	p.logger = hclog2slog.New(logger)
}

func (p *Plugin) Configure(_ context.Context, req *configv1.ConfigureRequest) (*configv1.ConfigureResponse, error) {
	slog.Info("Configuring plugin")

	return &configv1.ConfigureResponse{
		BuildInfo: &p.buildInfo,
	}, nil
}

func (p *Plugin) GetGroup(
	_ context.Context,
	_ *idmangv1.GetGroupRequest,
) (*idmangv1.GetGroupResponse, error) {
	return &idmangv1.GetGroupResponse{}, nil
}

func (p *Plugin) GetAllGroups(
	_ context.Context,
	_ *idmangv1.GetAllGroupsRequest,
) (*idmangv1.GetAllGroupsResponse, error) {
	return &idmangv1.GetAllGroupsResponse{}, nil
}

func (p *Plugin) GetUsersForGroup(
	_ context.Context,
	_ *idmangv1.GetUsersForGroupRequest,
) (*idmangv1.GetUsersForGroupResponse, error) {
	return &idmangv1.GetUsersForGroupResponse{}, nil
}

func (p *Plugin) GetGroupsForUser(
	_ context.Context,
	_ *idmangv1.GetGroupsForUserRequest,
) (*idmangv1.GetGroupsForUserResponse, error) {
	return &idmangv1.GetGroupsForUserResponse{}, nil
}
