package group

import (
	"github.com/google/uuid"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/utils/ptr"
	"github.tools.sap/kms/cmk/utils/sanitise"
)

func ToAPI(group model.Group) (*cmkapi.Group, error) {
	err := sanitise.Stringlikes(&group)
	if err != nil {
		return nil, err
	}

	return &cmkapi.Group{
		Name:          group.Name,
		Role:          cmkapi.GroupRole(group.Role),
		Description:   &group.Description,
		Id:            &group.ID,
		IamIdentifier: &group.IAMIdentifier,
	}, nil
}

func FromAPI(apiGroup cmkapi.Group, tenantID string) *model.Group {
	group := model.Group{
		Name:          apiGroup.Name,
		Role:          constants.Role(apiGroup.Role),
		Description:   ptr.GetSafeDeref(apiGroup.Description),
		ID:            uuid.New(),
		IAMIdentifier: model.NewIAMIdentifier(apiGroup.Name, tenantID),
	}

	return &group
}
