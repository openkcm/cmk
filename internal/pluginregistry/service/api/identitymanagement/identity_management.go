package identitymanagement

import (
	"context"

	"github.com/openkcm/plugin-sdk/api"
)

type IdentityManagement interface {
	ServiceInfo() api.Info

	GetGroup(ctx context.Context, req *GetGroupRequest) (*GetGroupResponse, error)
	ListGroups(ctx context.Context, req *ListGroupsRequest) (*ListGroupsResponse, error)
	ListGroupUsers(ctx context.Context, req *ListGroupUsersRequest) (*ListGroupUsersResponse, error)
	ListUserGroups(ctx context.Context, req *ListUserGroupsRequest) (*ListUserGroupsResponse, error)
}

type AuthContext struct {
	// V1 Fields
	Data map[string]string
}

type GetGroupRequest struct {
	// V1 Fields
	GroupName   string
	AuthContext AuthContext
}

type GetGroupResponse struct {
	// V1 Fields
	Group Group
}

type ListGroupsRequest struct {
	// V1 Fields
	AuthContext AuthContext
}

type ListGroupsResponse struct {
	// V1 Fields
	Groups []Group
}

type ListGroupUsersRequest struct {
	// V1 Fields
	GroupID     string
	AuthContext AuthContext
}

type ListGroupUsersResponse struct {
	// V1 Fields
	Users []User
}

type User struct {
	// V1 Fields
	ID    string
	Name  string
	Email string
}

type ListUserGroupsRequest struct {
	// V1 Fields
	UserID      string
	AuthContext AuthContext
}

type ListUserGroupsResponse struct {
	// V1 Fields
	Groups []Group
}

type Group struct {
	// V1 Fields
	ID   string
	Name string
}
