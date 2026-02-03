package cmk

import (
	"context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
)

func (c *APIController) GetUserInfo(
	ctx context.Context,
	_ cmkapi.GetUserInfoRequestObject,
) (cmkapi.GetUserInfoResponseObject, error) {
	userInfo, err := c.Manager.User.GetUserInfo(ctx)
	if err != nil {
		return nil, err
	}

	response := cmkapi.GetUserInfo200JSONResponse{
		Identifier: userInfo.Identifier,
		Email:      userInfo.Email,
		GivenName:  userInfo.GivenName,
		FamilyName: userInfo.FamilyName,
		Role:       userInfo.Role,
	}

	return response, nil
}
