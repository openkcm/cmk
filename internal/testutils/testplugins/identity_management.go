package testplugins

import (
	"context"

	"github.com/openkcm/plugin-sdk/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	servicewrapper "github.com/openkcm/cmk/internal/pluginregistry/service/wrapper"
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

type TestIdentityManagement struct{}

var _ identitymanagement.IdentityManagement = (*TestIdentityManagement)(nil)

func NewTestIdentityManagement() *TestIdentityManagement {
	return &TestIdentityManagement{}
}

func (s *TestIdentityManagement) ServiceInfo() api.Info {
	return testInfo{
		configuredType: servicewrapper.IdentityManagementServiceType,
	}
}

func (s *TestIdentityManagement) GetGroup(
	_ context.Context,
	req *identitymanagement.GetGroupRequest,
) (*identitymanagement.GetGroupResponse, error) {
	if g, ok := IdentityManagementGroups[req.GroupName]; ok {
		return &identitymanagement.GetGroupResponse{
			Group: identitymanagement.Group{
				ID:   g,
				Name: req.GroupName,
			},
		}, nil
	}
	return nil, status.New(codes.NotFound, "group does not exist").Err()
}

func (s *TestIdentityManagement) GetUser(
	_ context.Context,
	_ *identitymanagement.GetUserRequest,
) (*identitymanagement.GetUserResponse, error) {
	return &identitymanagement.GetUserResponse{}, nil
}

func (s *TestIdentityManagement) ListGroups(
	_ context.Context,
	_ *identitymanagement.ListGroupsRequest,
) (*identitymanagement.ListGroupsResponse, error) {
	return &identitymanagement.ListGroupsResponse{}, nil
}

func (s *TestIdentityManagement) ListGroupUsers(
	_ context.Context,
	req *identitymanagement.ListGroupUsersRequest,
) (*identitymanagement.ListGroupUsersResponse, error) {
	members, ok := IdentityManagementGroupMembership[req.GroupID]
	if !ok {
		return &identitymanagement.ListGroupUsersResponse{}, nil
	}

	users := make([]identitymanagement.User, 0, len(members))
	for _, u := range members {
		users = append(users, identitymanagement.User{
			ID:    u.ID,
			Email: u.Email,
		})
	}
	return &identitymanagement.ListGroupUsersResponse{Users: users}, nil
}

func (s *TestIdentityManagement) ListUserGroups(
	_ context.Context,
	_ *identitymanagement.ListUserGroupsRequest,
) (*identitymanagement.ListUserGroupsResponse, error) {
	return &identitymanagement.ListUserGroupsResponse{}, nil
}
