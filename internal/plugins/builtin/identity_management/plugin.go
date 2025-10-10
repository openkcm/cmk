package identity_management

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/openkcm/plugin-sdk/pkg/hclog2slog"
	identity_managementv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
	slogctx "github.com/veqryn/slog-context"
)

const (
	pluginName = "identity-management-empty"
)

func V1BuiltIn() catalog.BuiltIn {
	return builtin(&V1Plugin{})
}

func builtin(p *V1Plugin) catalog.BuiltIn {
	return catalog.MakeBuiltIn(pluginName,
		identity_managementv1.IdentityManagementServicePluginServer(p),
		configv1.ConfigServiceServer(p))
}

type V1Plugin struct {
	configv1.UnsafeConfigServer
	identity_managementv1.UnimplementedIdentityManagementServiceServer
}

var (
	_ identity_managementv1.IdentityManagementServiceServer = (*V1Plugin)(nil)
	_ configv1.ConfigServer                                 = (*V1Plugin)(nil)
)

// SetLogger method is called whenever the plugin start and giving the logger of host application
func (p *V1Plugin) SetLogger(logger hclog.Logger) {
	slog.SetDefault(hclog2slog.New(logger))
}

// Configure configures the plugin with the given configuration
func (p *V1Plugin) Configure(ctx context.Context, _ *configv1.ConfigureRequest) (*configv1.ConfigureResponse, error) {
	slogctx.Debug(ctx, "Builtin Certificate Issuer Service (cis) plugin")

	return &configv1.ConfigureResponse{}, nil
}

func (p *V1Plugin) GetAllGroups(ctx context.Context, in *identity_managementv1.GetAllGroupsRequest) (*identity_managementv1.GetAllGroupsResponse, error) {
	slogctx.Debug(ctx, "Builtin Identity Management Service (IMS) - GetAllGroups called")

	return &identity_managementv1.GetAllGroupsResponse{
		Groups: []*identity_managementv1.Group{},
	}, nil
}

func (p *V1Plugin) GetUsersForGroup(ctx context.Context, in *identity_managementv1.GetUsersForGroupRequest) (*identity_managementv1.GetUsersForGroupResponse, error) {
	slogctx.Debug(ctx, "Builtin Identity Management Service (IMS) - GetUsersForGroup called", "group", in.GetGroupId())

	return &identity_managementv1.GetUsersForGroupResponse{
		Users: []*identity_managementv1.User{},
	}, nil
}
func (p *V1Plugin) GetGroupsForUser(ctx context.Context, in *identity_managementv1.GetGroupsForUserRequest) (*identity_managementv1.GetGroupsForUserResponse, error) {
	slogctx.Debug(ctx, "Builtin Identity Management Service (IMS) - GetGroupsForUser called", "user", in.GetUserId())

	return &identity_managementv1.GetGroupsForUserResponse{
		Groups: []*identity_managementv1.Group{},
	}, nil
}
