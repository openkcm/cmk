package testplugins

import (
	"context"
	"log/slog"

	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	idmangv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
)

type IdentityManagementUserRef struct {
	ID    string
	Email string
}

var IdentityManagementGroups = map[string]string{
	"KMS_001": "SCIM-Group-ID-001",
	"KMS_002": "SCIM-Group-ID-002",
}

var IdentityManagementGroupMembership = map[string][]IdentityManagementUserRef{
	"SCIM-Group-ID-001": {
		{"00000000-0000-0000-0000-100000000001", "user1@example.com"},
		{"00000000-0000-0000-0000-100000000002", "user2@example.com"},
	},
	"SCIM-Group-ID-002": {
		{"00000000-0000-0000-0000-100000000003", "user3@example.com"},
		{"00000000-0000-0000-0000-100000000004", "user4@example.com"},
	},
}

type IdentityManagement struct {
	configv1.UnsafeConfigServer
	idmangv1.UnsafeIdentityManagementServiceServer
}

func NewIdentityManagement() catalog.BuiltInPlugin {
	p := &IdentityManagement{}
	return catalog.MakeBuiltIn(
		Name,
		idmangv1.IdentityManagementServicePluginServer(p),
		configv1.ConfigServiceServer(p),
	)
}

func (p *IdentityManagement) GetUser(
	_ context.Context,
	_ *idmangv1.GetUserRequest,
) (*idmangv1.GetUserResponse, error) {
	return &idmangv1.GetUserResponse{}, nil
}

func (p *IdentityManagement) Configure(
	_ context.Context,
	_ *configv1.ConfigureRequest,
) (*configv1.ConfigureResponse, error) {
	slog.Info("Configuring plugin")

	return &configv1.ConfigureResponse{}, nil
}

func (p *IdentityManagement) GetUsersForGroup(
	_ context.Context,
	req *idmangv1.GetUsersForGroupRequest,
) (*idmangv1.GetUsersForGroupResponse, error) {
	users, ok := IdentityManagementGroupMembership[req.GetGroupId()]
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

func (p *IdentityManagement) GetGroupsForUser(
	_ context.Context,
	_ *idmangv1.GetGroupsForUserRequest,
) (*idmangv1.GetGroupsForUserResponse, error) {
	return &idmangv1.GetGroupsForUserResponse{}, nil
}

func (p *IdentityManagement) GetAllGroups(
	_ context.Context,
	_ *idmangv1.GetAllGroupsRequest,
) (*idmangv1.GetAllGroupsResponse, error) {
	return &idmangv1.GetAllGroupsResponse{}, nil
}

func (p *IdentityManagement) GetGroup(
	_ context.Context,
	req *idmangv1.GetGroupRequest,
) (*idmangv1.GetGroupResponse, error) {
	if g, ok := IdentityManagementGroups[req.GetGroupName()]; ok {
		return &idmangv1.GetGroupResponse{
			Group: &idmangv1.Group{
				Id:   g,
				Name: req.GetGroupName(),
			},
		}, nil
	}

	return nil, status.New(codes.NotFound, "group does not exist").Err()
}
