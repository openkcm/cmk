package testutils

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
)

type user struct{}

func NewUserManager() manager.User {
	return &user{}
}

func (u *user) NeedsGroupFiltering(
	ctx context.Context,
	action authz.Action,
	resource authz.ResourceTypeName,
) (bool, error) {
	return false, nil
}

func (u *user) HasTenantAccess(ctx context.Context) (bool, error) {
	return true, nil
}

func (u *user) HasSystemAccess(ctx context.Context, action authz.Action, system *model.System) (bool, error) {
	return false, nil
}

func (u *user) HasKeyAccess(ctx context.Context, action authz.Action, keyConfig uuid.UUID) (bool, error) {
	return false, nil
}

func (u *user) HasKeyConfigAccess(
	ctx context.Context,
	action authz.Action,
	keyConfig *model.KeyConfiguration,
) (bool, error) {
	return false, nil
}

func (u *user) GetRoleFromIAM(ctx context.Context, iamIdentifiers []string) (constants.Role, error) {
	return constants.KeyAdminRole, nil
}

func (u *user) GetUserInfo(ctx context.Context) (manager.UserInfo, error) {
	return manager.UserInfo{}, nil
}
