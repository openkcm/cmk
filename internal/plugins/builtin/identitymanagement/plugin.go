package identitymanagement

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/openkcm/plugin-sdk/pkg/hclog2slog"

	identitymanagementv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"
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
		identitymanagementv1.IdentityManagementServicePluginServer(p),
		configv1.ConfigServiceServer(p))
}

type V1Plugin struct {
	configv1.UnsafeConfigServer
	identitymanagementv1.UnimplementedIdentityManagementServiceServer
}

var (
	_ identitymanagementv1.IdentityManagementServiceServer = (*V1Plugin)(nil)
	_ configv1.ConfigServer                                = (*V1Plugin)(nil)
)

// SetLogger method is called whenever the plugin start and giving the logger of host application
func (p *V1Plugin) SetLogger(logger hclog.Logger) {
	slog.SetDefault(hclog2slog.New(logger))
}

// Configure configures the plugin with the given configuration
func (p *V1Plugin) Configure(
	ctx context.Context,
	_ *configv1.ConfigureRequest,
) (*configv1.ConfigureResponse, error) {
	slogctx.Debug(ctx, "Builtin Certificate Issuer Service (cis) plugin")

	return &configv1.ConfigureResponse{}, nil
}

func (p *V1Plugin) GetAllGroups(
	ctx context.Context,
	_ *identitymanagementv1.GetAllGroupsRequest,
) (*identitymanagementv1.GetAllGroupsResponse, error) {
	slogctx.Debug(ctx, "Builtin Identity Management Service (IMS) - GetAllGroups called")

	return &identitymanagementv1.GetAllGroupsResponse{
		Groups: []*identitymanagementv1.Group{},
	}, nil
}

func (p *V1Plugin) GetUsersForGroup(
	ctx context.Context,
	in *identitymanagementv1.GetUsersForGroupRequest,
) (*identitymanagementv1.GetUsersForGroupResponse, error) {
	slogctx.Debug(ctx, "Builtin Identity Management Service (IMS) - GetUsersForGroup called", "group", in.GetGroupId())

	return &identitymanagementv1.GetUsersForGroupResponse{
		Users: []*identitymanagementv1.User{},
	}, nil
}
func (p *V1Plugin) GetGroupsForUser(
	ctx context.Context,
	in *identitymanagementv1.GetGroupsForUserRequest,
) (*identitymanagementv1.GetGroupsForUserResponse, error) {
	slogctx.Debug(ctx, "Builtin Identity Management Service (IMS) - GetGroupsForUser called", "user", in.GetUserId())

	return &identitymanagementv1.GetGroupsForUserResponse{
		Groups: []*identitymanagementv1.Group{},
	}, nil
}
