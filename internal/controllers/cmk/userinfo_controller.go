package cmk

import (
	"context"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

func (c *APIController) GetUserInfo(
	ctx context.Context,
	_ cmkapi.GetUserInfoRequestObject,
) (cmkapi.GetUserInfoResponseObject, error) {
	clientData, err := cmkcontext.ExtractClientData(ctx)
	if err != nil {
		return nil, err
	}

	groups, err := cmkcontext.ExtractClientDataGroups(ctx)
	if err != nil {
		return nil, err
	}

	roles, err := c.Manager.Group.GetRoleFromGroupIAMIdentifiers(ctx, groups)
	if err != nil {
		return nil, err
	}

	response := cmkapi.GetUserInfo200JSONResponse{
		Identifier: clientData.Identifier,
		Email:      clientData.Email,
		GivenName:  clientData.GivenName,
		FamilyName: clientData.FamilyName,
		Role:       string(roles),
	}

	return response, nil
}
