package group

import (
	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
)

func ToAPI(group model.Group) *cmkapi.Group {
	return &cmkapi.Group{
		Name:          group.Name,
		Role:          cmkapi.GroupRole(group.Role),
		Description:   &group.Description,
		Id:            &group.ID,
		IamIdentifier: &group.IAMIdentifier,
	}
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
