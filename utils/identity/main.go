package identity

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	cmkContext "github.com/openkcm/cmk/utils/context"
)

func GetUserName(
	ctx context.Context,
	identityManager identitymanagement.IdentityManagement,
	id string,
) (string, error) {
	authCtx, err := cmkContext.ExtractBusinessUserDataAuthContext(ctx)
	if err != nil {
		return "", err
	}
	user, err := identityManager.GetUser(ctx, &identitymanagement.GetUserRequest{
		UserID:      id,
		AuthContext: identitymanagement.AuthContext{Data: authCtx},
	})

	grpcErr := status.Convert(err)
	// Could not find user in IAM
	if grpcErr.Code() == codes.NotFound {
		return constants.UnknownUserName, nil
	}

	if err != nil {
		return "", err
	}

	return user.User.Email, nil
}
