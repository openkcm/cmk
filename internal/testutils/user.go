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
	action authz.APIAction,
	resource authz.APIResourceTypeName,
) (bool, error) {
	return false, nil
}

func (u *user) HasTenantAccess(ctx context.Context) (bool, error) {
	return true, nil
}

func (u *user) HasSystemAccess(ctx context.Context, action authz.APIAction, system *model.System) (bool, error) {
	return false, nil
}

func (u *user) HasKeyAccess(ctx context.Context, action authz.APIAction, keyConfig uuid.UUID) (bool, error) {
	return false, nil
}

func (u *user) HasKeyConfigAccess(
	ctx context.Context,
	action authz.APIAction,
	keyConfig *model.KeyConfiguration,
) (bool, error) {
	return false, nil
}

func (u *user) GetRoleFromIAM(ctx context.Context, iamIdentifiers []string) (constants.BusinessRole, error) {
	return constants.KeyAdminRole, nil
}

func (u *user) GetUserInfo(ctx context.Context) (manager.UserInfo, error) {
	return manager.UserInfo{}, nil
}
