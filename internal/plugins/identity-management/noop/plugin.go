package noop

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/openkcm/plugin-sdk/pkg/hclog2slog"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

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

type Plugin struct {
	idmangv1.UnsafeIdentityManagementServiceServer
	configv1.UnsafeConfigServer

	logger    *slog.Logger
	buildInfo string

	staticConfiguration *StaticIdentityManagement
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

	cfg := Config{}

	err := yaml.Unmarshal([]byte(req.GetYamlConfiguration()), &cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get yaml Configuration")
	}

	content, err := commoncfg.ExtractValueFromSourceRef(&cfg.StaticJsonContent)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get yaml configuration")
	}

	p.staticConfiguration = &StaticIdentityManagement{}
	err = yaml.Unmarshal(content, p.staticConfiguration)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get yaml static configuration")
	}

	return &configv1.ConfigureResponse{
		BuildInfo: &p.buildInfo,
	}, nil
}

func (p *Plugin) GetGroup(
	_ context.Context,
	req *idmangv1.GetGroupRequest,
) (*idmangv1.GetGroupResponse, error) {
	for _, group := range p.staticConfiguration.Groups {
		if group.Name == req.GroupName {
			return &idmangv1.GetGroupResponse{
				Group: &idmangv1.Group{
					Id:   group.ID,
					Name: group.Name,
				},
			}, nil
		}
	}
	return &idmangv1.GetGroupResponse{}, nil
}

func (p *Plugin) GetAllGroups(
	_ context.Context,
	req *idmangv1.GetAllGroupsRequest,
) (*idmangv1.GetAllGroupsResponse, error) {
	groups := make([]*idmangv1.Group, 0, len(p.staticConfiguration.Groups))
	for _, group := range p.staticConfiguration.Groups {
		groups = append(groups, &idmangv1.Group{
			Id:   group.ID,
			Name: group.Name,
		})
	}
	return &idmangv1.GetAllGroupsResponse{
		Groups: groups,
	}, nil
}

func (p *Plugin) GetUsersForGroup(
	_ context.Context,
	req *idmangv1.GetUsersForGroupRequest,
) (*idmangv1.GetUsersForGroupResponse, error) {
	users := make([]*idmangv1.User, 0)
	for _, group := range p.staticConfiguration.Groups {
		if group.ID == req.GroupId {
			for _, user := range group.Users {
				users = append(users, &idmangv1.User{
					Id:    user.ID,
					Name:  user.Name,
					Email: user.Email,
				})
			}
		}
	}
	return &idmangv1.GetUsersForGroupResponse{
		Users: users,
	}, nil
}

func (p *Plugin) GetGroupsForUser(
	_ context.Context,
	req *idmangv1.GetGroupsForUserRequest,
) (*idmangv1.GetGroupsForUserResponse, error) {
	groups := make([]*idmangv1.Group, 0, len(p.staticConfiguration.Groups))
	for _, group := range p.staticConfiguration.Groups {
		for _, user := range group.Users {
			if user.ID == req.UserId {
				groups = append(groups, &idmangv1.Group{
					Id:   group.ID,
					Name: group.Name,
				})
			}
		}
	}
	return &idmangv1.GetGroupsForUserResponse{
		Groups: groups,
	}, nil
}
