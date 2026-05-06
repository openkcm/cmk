package testplugins

import (
	"context"
	"fmt"
	"maps"

	"github.com/google/uuid"
	"github.com/openkcm/plugin-sdk/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	servicewrapper "github.com/openkcm/cmk/internal/pluginregistry/service/wrapper"
)

// IdentityManagementGroups is the default global group name→SCIM-ID mapping used by NewTestIdentityManagement.
var IdentityManagementGroups = map[string]string{
	"KMS_001": "SCIM-Group-ID-001",
	"KMS_002": "SCIM-Group-ID-002",
}

// IdentityManagementGroupMembership is the default global SCIM group ID→member user ID
// list mapping used by NewTestIdentityManagement.
var IdentityManagementGroupMembership = map[string][]string{
	"SCIM-Group-ID-001": {
		"00000000-0000-0000-0000-100000000001",
		"00000000-0000-0000-0000-100000000002",
	},
	"SCIM-Group-ID-002": {
		"00000000-0000-0000-0000-100000000003",
		"00000000-0000-0000-0000-100000000004",
	},
}

// IdentityManagementUsers is the default global user ID→User mapping used by NewTestIdentityManagement.
var IdentityManagementUsers = map[string]identitymanagement.User{
	"00000000-0000-0000-0000-100000000001": {
		ID:    "00000000-0000-0000-0000-100000000001",
		Name:  "user1@example.com",
		Email: "user1@example.com",
	},
	"00000000-0000-0000-0000-100000000002": {
		ID:    "00000000-0000-0000-0000-100000000002",
		Name:  "user2@example.com",
		Email: "user2@example.com",
	},
	"00000000-0000-0000-0000-100000000003": {
		ID:    "00000000-0000-0000-0000-100000000003",
		Name:  "user3@example.com",
		Email: "user3@example.com",
	},
	"00000000-0000-0000-0000-100000000004": {
		ID:    "00000000-0000-0000-0000-100000000004",
		Name:  "user4@example.com",
		Email: "user4@example.com",
	},
}

// IdentityManagementOption configures a TestIdentityManagement instance.
type IdentityManagementOption func(*TestIdentityManagement)

func WithGroups(groups map[string]string) IdentityManagementOption {
	return func(s *TestIdentityManagement) {
		s.groups = groups
	}
}

func WithGroupMembership(membership map[string][]string) IdentityManagementOption {
	return func(s *TestIdentityManagement) {
		s.groupMembership = membership
	}
}

func WithUsers(users []identitymanagement.User) IdentityManagementOption {
	return func(s *TestIdentityManagement) {
		s.users = make(map[string]identitymanagement.User, len(users))
		for _, u := range users {
			s.users[u.ID] = u
		}
	}
}

// TestIdentityManagement is a native Go test double for the identity management plugin.
// Each instance holds its own maps so tests remain isolated.
type TestIdentityManagement struct {
	groups          map[string]string
	groupMembership map[string][]string
	users           map[string]identitymanagement.User
}

var _ identitymanagement.IdentityManagement = (*TestIdentityManagement)(nil)

// NewTestIdentityManagement returns a new instance pre-populated from the global
// default fixtures. Pass IdentityManagementOption to override with custom data.
func NewTestIdentityManagement(opts ...IdentityManagementOption) *TestIdentityManagement {
	s := &TestIdentityManagement{
		groups:          maps.Clone(IdentityManagementGroups),
		groupMembership: maps.Clone(IdentityManagementGroupMembership),
		users:           maps.Clone(IdentityManagementUsers),
	}
	for _, o := range opts {
		o(s)
	}
	return s
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
	if g, ok := s.groups[req.GroupName]; ok {
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
	req *identitymanagement.GetUserRequest,
) (*identitymanagement.GetUserResponse, error) {
	u, ok := s.users[req.UserID]
	if !ok {
		return nil, status.New(codes.NotFound, "user does not exist").Err()
	}
	return &identitymanagement.GetUserResponse{User: u}, nil
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
	memberIDs, ok := s.groupMembership[req.GroupID]
	if !ok {
		return &identitymanagement.ListGroupUsersResponse{}, nil
	}

	users := make([]identitymanagement.User, 0, len(memberIDs))
	for _, id := range memberIDs {
		u, ok := s.users[id]
		if !ok {
			return nil, status.New(codes.NotFound, fmt.Sprintf("user %s not found in group %s", id, req.GroupID)).Err()
		}
		users = append(users, u)
	}
	return &identitymanagement.ListGroupUsersResponse{Users: users}, nil
}

func (s *TestIdentityManagement) ListUserGroups(
	_ context.Context,
	_ *identitymanagement.ListUserGroupsRequest,
) (*identitymanagement.ListUserGroupsResponse, error) {
	return &identitymanagement.ListUserGroupsResponse{}, nil
}

// PutGroup registers a group name→SCIM-ID mapping.
func (s *TestIdentityManagement) PutGroup(name, scimID string) {
	s.groups[name] = scimID
}

// DeleteGroup removes a group name→SCIM-ID mapping, simulating a SCIM lookup failure.
func (s *TestIdentityManagement) DeleteGroup(name string) {
	delete(s.groups, name)
}

// PutGroupMembers replaces the member list for the given SCIM group ID.
// Panics if the SCIM group ID is not registered via PutGroup or any user ID is not registered via PutUser.
func (s *TestIdentityManagement) PutGroupMembers(scimID string, userIDs []string) {
	scimKnown := false
	for _, id := range s.groups {
		if id == scimID {
			scimKnown = true
			break
		}
	}
	if !scimKnown {
		panic(fmt.Sprintf("testplugins: PutGroupMembers: SCIM group %q is not registered; call PutGroup first", scimID))
	}
	for _, id := range userIDs {
		if _, ok := s.users[id]; !ok {
			panic(fmt.Sprintf("testplugins: PutGroupMembers: user %q is not registered; call PutUser first", id))
		}
	}
	s.groupMembership[scimID] = userIDs
}

// PutUser registers a user fixture returned by GetUser. The user is keyed by user.ID.
// If Name is empty it defaults to user.ID + "@example.com".
// If Email is empty it defaults to Name.
func (s *TestIdentityManagement) PutUser(user identitymanagement.User) {
	if user.ID == "" {
		user.ID = uuid.NewString()
	}
	if user.Name == "" {
		user.Name = user.ID + "@example.com"
	}
	if user.Email == "" {
		user.Email = user.Name
	}
	s.users[user.ID] = user
}
