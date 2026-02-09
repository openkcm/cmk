package scim

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/identity-management-plugins/pkg/clients/scim"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/openkcm/plugin-sdk/pkg/hclog2slog"
	"github.com/samber/oops"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"

	idmangv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/plugins/identity-management/scim/client"
	"github.com/openkcm/cmk/internal/plugins/identity-management/scim/config"
)

const (
	defaultListMethod = http.MethodPost

	defaultUserListAttribute     = "groups.display"
	defaultGroupsFilterAttribute = "displayName"

	modifiedByAttribute = "meta.lastModified"
)

var (
	ErrID                     = oops.In("Identity management Plugin")
	ErrNoScimClient           = errors.New("no scim client exists")
	ErrGetGroup               = errors.New("failed to get group")
	ErrGetAllGroups           = errors.New("failed to get allx group")
	ErrGetGroupNonExistent    = status.New(codes.NotFound, "group does not exist").Err()
	ErrGetGroupMultipleGroups = errors.New("more than one group")
	ErrGetGroupsForUser       = errors.New("failed to get groups for user")
	ErrGetUsersForGroup       = errors.New("failed to get users for group")
	ErrNoID                   = errors.New("no filter id provided")
	ErrNoGroupAttribute       = errors.New("no group attribute configured")
)

// allFilter is used to get all users or groups
// by comparing the modified time to the zero timestamp
var allFilter = scim.FilterComparison{
	Attribute: modifiedByAttribute,
	Operator:  scim.FilterOperatorGreater,
	Value:     time.Unix(0, 0).Format(time.RFC3339),
}

type Params struct {
	BaseHost                string // Fallback host if not provided in auth context
	GroupAttribute          string
	UserAttribute           string
	GroupMembersAttribute   string
	ListMethod              string
	AllowSearchUsersByGroup bool
	AuthContext             config.AuthContextConfig
}

const (
	pluginName = "SYSINFO"
)

func Register(registry catalog.BuiltInPluginRegistry) {
	registry.Register(builtin(NewPlugin()))
}

func builtin(p *Plugin) catalog.BuiltInPlugin {
	return catalog.AsBuiltIn(pluginName,
		idmangv1.IdentityManagementServicePluginServer(p),
		configv1.ConfigServiceServer(p))
}

// Plugin is a simple test implementation of KeystoreProviderServer
type Plugin struct {
	idmangv1.UnsafeIdentityManagementServiceServer
	configv1.UnsafeConfigServer

	logger     hclog.Logger
	scimClient *client.Client
	params     *Params
	buildInfo  string
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
	p.logger = logger // Keep a copy of the logger for client creation
	slog.SetDefault(hclog2slog.New(logger))
}

//nolint:gci
func (p *Plugin) Configure(_ context.Context, req *configv1.ConfigureRequest) (*configv1.ConfigureResponse, error) {
	slog.Info("Configuring plugin")

	cfg := &config.Config{}

	err := yaml.Unmarshal([]byte(req.GetYamlConfiguration()), cfg)
	if err != nil {
		return nil, ErrID.Wrapf(err, "Failed to get yaml Configuration")
	}

	params, err := createParams(cfg)
	if err != nil {
		return nil, err
	}

	p.params = params

	client, err := client.NewClient(cfg.Auth, p.logger)
	if err != nil {
		return nil, err
	}

	p.scimClient = client

	return &configv1.ConfigureResponse{
		BuildInfo: &p.buildInfo,
	}, nil
}

func (p *Plugin) GetGroup(
	ctx context.Context,
	request *idmangv1.GetGroupRequest,
) (*idmangv1.GetGroupResponse, error) {
	if p.scimClient == nil {
		return nil, ErrNoScimClient
	}

	attr := p.params.GroupAttribute
	filter := getFilter(defaultGroupsFilterAttribute, request.GetGroupName(), attr)

	responseGroups, err := p.listGroups(ctx, filter, request.GetAuthContext().GetData())
	if err != nil {
		p.logger.Error("GetGroup: error listing groups", "error", err)
		return nil, errs.Wrap(ErrGetGroup, err)
	}

	if len(responseGroups) == 0 {
		return nil, ErrGetGroupNonExistent
	} else if len(responseGroups) > 1 {
		return nil, errs.Wrap(ErrGetGroup, ErrGetGroupMultipleGroups)
	}

	return &idmangv1.GetGroupResponse{Group: responseGroups[0]}, nil
}

func (p *Plugin) GetAllGroups(
	ctx context.Context,
	request *idmangv1.GetAllGroupsRequest,
) (*idmangv1.GetAllGroupsResponse, error) {
	host, headers := p.extractAuthContext(request.GetAuthContext().GetData())

	groups, err := p.scimClient.ListGroups(ctx, client.RequestParams{
		Host:    host,
		Method:  p.getListMethod(),
		Filter:  allFilter,
		Headers: headers,
	})
	if err != nil {
		return nil, errs.Wrap(ErrGetAllGroups, err)
	}

	responseGroups := make([]*idmangv1.Group, len(groups.Resources))

	for i, group := range groups.Resources {
		responseGroups[i] = &idmangv1.Group{Id: group.ID,
			Name: group.DisplayName}
	}

	return &idmangv1.GetAllGroupsResponse{Groups: responseGroups}, nil
}

func (p *Plugin) GetUsersForGroup(
	ctx context.Context,
	request *idmangv1.GetUsersForGroupRequest,
) (*idmangv1.GetUsersForGroupResponse, error) {
	if p.scimClient == nil {
		return nil, ErrNoScimClient
	}

	groupID := request.GetGroupId()

	if groupID == "" {
		return nil, errs.Wrap(ErrGetUsersForGroup, ErrNoID)
	}

	var (
		responseUsers        []*idmangv1.User
		getUsersForGroupFunc func(context.Context, string, string, map[string]string) ([]*idmangv1.User, error)
	)

	if p.params.AllowSearchUsersByGroup {
		getUsersForGroupFunc = p.getUsersForGroupUsingUserList
	} else {
		// If SCIM API does not support filtering users by group attribute,
		// we need to fall back to getting individual users by firstly
		// getting the user IDs from the group members attribute and
		// then getting each user by their ID.
		getUsersForGroupFunc = p.getUsersForGroupUsingGroupMembers
	}

	host, headers := p.extractAuthContext(request.GetAuthContext().GetData())

	responseUsers, err := getUsersForGroupFunc(ctx, groupID, host, headers)
	if err != nil {
		return nil, errs.Wrap(ErrGetUsersForGroup, err)
	}

	return &idmangv1.GetUsersForGroupResponse{Users: responseUsers}, nil
}

func (p *Plugin) GetGroupsForUser(
	ctx context.Context,
	request *idmangv1.GetGroupsForUserRequest,
) (*idmangv1.GetGroupsForUserResponse, error) {
	if p.scimClient == nil {
		return nil, ErrNoScimClient
	}

	attr := p.params.UserAttribute
	filter := getFilter(defaultUserListAttribute, request.GetUserId(), attr)

	responseGroups, err := p.listGroups(ctx, filter, request.GetAuthContext().GetData())
	if err != nil {
		return nil, errs.Wrap(ErrGetGroupsForUser, err)
	}

	return &idmangv1.GetGroupsForUserResponse{Groups: responseGroups}, nil
}

func (p *Plugin) listGroups(
	ctx context.Context,
	filter scim.FilterExpression,
	authContextData map[string]string,
) ([]*idmangv1.Group, error) {
	if (filter == scim.NullFilterExpression{}) {
		return nil, ErrNoID
	}

	host, headers := p.extractAuthContext(authContextData)

	groups, err := p.scimClient.ListGroups(ctx, client.RequestParams{
		Host:    host,
		Method:  p.getListMethod(),
		Filter:  filter,
		Headers: headers,
	})
	if err != nil {
		return nil, err
	}

	responseGroups := make([]*idmangv1.Group, len(groups.Resources))

	for i, group := range groups.Resources {
		responseGroups[i] = &idmangv1.Group{Id: group.ID,
			Name: group.DisplayName}
	}

	return responseGroups, nil
}

func (p *Plugin) getListMethod() string {
	if p.params.ListMethod != "" {
		return p.params.ListMethod
	}

	return defaultListMethod
}

func (p *Plugin) getUsersForGroupUsingUserList(
	ctx context.Context,
	groupID string,
	host string,
	headers map[string]string,
) ([]*idmangv1.User, error) {
	responseUsers := make([]*idmangv1.User, 0)

	attr := p.params.GroupAttribute
	if attr == "" {
		return nil, errs.Wrap(ErrGetUsersForGroup, ErrNoGroupAttribute)
	}

	filter := getFilter(defaultUserListAttribute, groupID, attr)

	users, err := p.scimClient.ListUsers(ctx, client.RequestParams{
		Host:    host,
		Method:  p.getListMethod(),
		Filter:  filter,
		Headers: headers,
	})
	if err != nil {
		return nil, errs.Wrap(ErrGetUsersForGroup, err)
	}

	for _, user := range users.Resources {
		responseUsers = append(responseUsers, &idmangv1.User{
			Id:    user.ID,
			Name:  user.UserName,
			Email: getPrimaryEmailAddress(&user),
		})
	}

	return responseUsers, nil
}

func (p *Plugin) getUsersForGroupUsingGroupMembers(
	ctx context.Context,
	groupID string,
	host string,
	headers map[string]string,
) ([]*idmangv1.User, error) {
	responseUsers := make([]*idmangv1.User, 0)

	group, err := p.scimClient.GetGroup(
		ctx, groupID, p.params.GroupMembersAttribute,
		client.RequestParams{
			Host:    host,
			Headers: headers,
		},
	)
	if err != nil {
		return nil, errs.Wrap(ErrGetUsersForGroup, err)
	}

	for _, member := range group.Members {
		user, err := p.scimClient.GetUser(ctx, member.Value, client.RequestParams{
			Host:    host,
			Headers: headers,
		})
		if err != nil {
			return nil, errs.Wrap(ErrGetUsersForGroup, err)
		}

		responseUsers = append(responseUsers, &idmangv1.User{
			Id:    user.ID,
			Name:  user.UserName,
			Email: getPrimaryEmailAddress(user),
		})
	}

	return responseUsers, nil
}

func (p *Plugin) extractAuthContext(authContextData map[string]string) (string, map[string]string) {
	hostField := p.params.AuthContext.HostField
	host := authContextData[hostField]

	if host != "" {
		joinedURL, err := url.JoinPath(host, p.params.AuthContext.BasePath)
		if err != nil {
			p.logger.Warn("Failed to join host and base path, using host as is",
				"error", err, "host", host, "basePath", p.params.AuthContext.BasePath)
		} else {
			host = joinedURL
		}
	} else {
		host = p.params.BaseHost
	}

	headers := make(map[string]string)

	for key, field := range p.params.AuthContext.HeaderFields {
		if val, ok := authContextData[field]; ok {
			headers[key] = val
		}
	}

	return host, headers
}

func getFilter(defaultAttribute, value string, setAttribute string) scim.FilterExpression {
	if value == "" {
		return scim.NullFilterExpression{}
	}

	filter := scim.FilterComparison{
		Attribute: defaultAttribute,
		Operator:  scim.FilterOperatorEqual,
		Value:     value,
	}

	if setAttribute != "" {
		filter.Attribute = setAttribute
	}

	return filter
}

func createParams(cfg *config.Config) (*Params, error) {
	baseHostBytes, err := commoncfg.LoadValueFromSourceRef(cfg.Host)
	if err != nil {
		return nil, ErrID.Wrapf(err, "Failed loading base host")
	}

	groupAttrBytes, err := commoncfg.LoadValueFromSourceRef(cfg.Params.GroupAttribute)
	if err != nil {
		return nil, ErrID.Wrapf(err, "Failed loading group attribute")
	}

	userAttrBytes, err := commoncfg.LoadValueFromSourceRef(cfg.Params.UserAttribute)
	if err != nil {
		return nil, ErrID.Wrapf(err, "Failed loading user attribute")
	}

	groupMemberAttrBytes, err := commoncfg.LoadValueFromSourceRef(cfg.Params.GroupMembersAttribute)
	if err != nil {
		return nil, ErrID.Wrapf(err, "Failed loading group members attribute")
	}

	listMethodBytes, err := commoncfg.LoadValueFromSourceRef(cfg.Params.ListMethod)
	if err != nil {
		return nil, ErrID.Wrapf(err, "Failed loading list method")
	}

	allowSearchUsersByGroupBytes, err := commoncfg.LoadValueFromSourceRef(cfg.Params.AllowSearchUsersByGroup)
	if err != nil {
		return nil, ErrID.Wrapf(err, "Failed loading allow search users by group")
	}

	allowSearchUsersByGroup, err := strconv.ParseBool(string(allowSearchUsersByGroupBytes))
	if err != nil {
		return nil, ErrID.Wrapf(err, "Failed parsing allow search users by group")
	}

	authContextBytes, err := commoncfg.LoadValueFromSourceRef(cfg.AuthContext)
	if err != nil {
		return nil, ErrID.Wrapf(err, "Failed loading auth context")
	}

	cfgAuthContext := config.AuthContextConfig{}

	err = yaml.Unmarshal(authContextBytes, &cfgAuthContext)
	if err != nil {
		return nil, ErrID.Wrapf(err, "Failed to unmarshal auth context")
	}

	return &Params{
		BaseHost:                string(baseHostBytes),
		GroupAttribute:          string(groupAttrBytes),
		UserAttribute:           string(userAttrBytes),
		GroupMembersAttribute:   string(groupMemberAttrBytes),
		ListMethod:              string(listMethodBytes),
		AllowSearchUsersByGroup: allowSearchUsersByGroup,
		AuthContext:             cfgAuthContext,
	}, nil
}

func getPrimaryEmailAddress(user *client.User) string {
	for _, email := range user.Emails {
		if email.Primary {
			return email.Value
		}
	}

	// Fallback to the first email if no primary is set
	if len(user.Emails) > 0 {
		return user.Emails[0].Value
	}

	return ""
}
