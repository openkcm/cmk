package testpluginregistry

import (
	"context"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	"github.com/openkcm/plugin-sdk/api"
)

type mockIDMService struct {
	identitymanagement.IdentityManagement

	// Override function for GetUser
	GetUserFn func(context.Context, *identitymanagement.GetUserRequest) (*identitymanagement.GetUserResponse, error)
}

func NewMockIDMService() mockIDMService {
	return mockIDMService{}
}

func (m mockIDMService) GetUser(
	ctx context.Context,
	req *identitymanagement.GetUserRequest,
) (*identitymanagement.GetUserResponse, error) {
	if m.GetUserFn != nil {
		return m.GetUserFn(ctx, req)
	}
	return &identitymanagement.GetUserResponse{
		User: identitymanagement.User{
			Name: "initiator@example.com",
		},
	}, nil
}

func (m mockIDMService) GetGroup(
	ctx context.Context,
	req *identitymanagement.GetGroupRequest,
) (*identitymanagement.GetGroupResponse, error) {
	panic("not implemented")
}

func (m mockIDMService) ListGroups(
	ctx context.Context,
	req *identitymanagement.ListGroupsRequest,
) (*identitymanagement.ListGroupsResponse, error) {
	panic("not implemented")
}

func (m mockIDMService) ListGroupUsers(
	ctx context.Context,
	req *identitymanagement.ListGroupUsersRequest,
) (*identitymanagement.ListGroupUsersResponse, error) {
	panic("not implemented")
}

func (m mockIDMService) ListUserGroups(
	ctx context.Context,
	req *identitymanagement.ListUserGroupsRequest,
) (*identitymanagement.ListUserGroupsResponse, error) {
	panic("not implemented")
}

func (m mockIDMService) ServiceInfo() api.Info {
	panic("not implemented")
}
