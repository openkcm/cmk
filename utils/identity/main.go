package identity

import (
	"context"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	cmkContext "github.com/openkcm/cmk/utils/context"
)

func GetUserName(
	ctx context.Context,
	identityManager identitymanagement.IdentityManagement,
	id string,
) (string, error) {
	authCtx, err := cmkContext.ExtractClientDataAuthContext(ctx)
	if err != nil {
		return "", err
	}
	user, err := identityManager.GetUser(ctx, &identitymanagement.GetUserRequest{
		UserID:      id,
		AuthContext: identitymanagement.AuthContext{Data: authCtx},
	})
	if err != nil {
		return "", err
	}
	return user.User.Name, nil
}
