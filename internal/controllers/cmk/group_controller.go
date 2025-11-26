package cmk

import (
	"context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform"
	tfGroup "github.com/openkcm/cmk/internal/api/transform/group"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

func (c *APIController) GetGroups(
	ctx context.Context,
	request cmkapi.GetGroupsRequestObject,
) (cmkapi.GetGroupsResponseObject, error) {
	skip := ptr.GetIntOrDefault(request.Params.Skip, constants.DefaultSkip)
	top := ptr.GetIntOrDefault(request.Params.Top, constants.DefaultTop)

	groups, total, err := c.Manager.Group.GetGroups(ctx, skip, top)
	if err != nil {
		return nil, err
	}

	values, err := transform.ToList(groups, func(group model.Group) (*cmkapi.Group, error) {
		apiGroup, err := tfGroup.ToAPI(group)
		if err != nil {
			return nil, err
		}

		return apiGroup, nil
	})

	response := cmkapi.GroupList{
		Value: values,
	}

	if ptr.GetSafeDeref(request.Params.Count) {
		response.Count = ptr.PointTo(total)
	}

	return cmkapi.GetGroups200JSONResponse(response), err
}

func (c *APIController) CreateGroup(
	ctx context.Context,
	request cmkapi.CreateGroupRequestObject,
) (cmkapi.CreateGroupResponseObject, error) {
	// This should only be checked if request comes from UI
	if request.Body.Role != cmkapi.GroupRoleKEYADMINISTRATOR {
		return nil, manager.ErrGroupRole
	}

	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return nil, err
	}

	group := tfGroup.FromAPI(*request.Body, tenantID)

	group, err = c.Manager.Group.CreateGroup(ctx, group)
	if err != nil {
		return nil, err
	}

	apiGroup, err := tfGroup.ToAPI(*group)
	if err != nil {
		return nil, err
	}

	return cmkapi.CreateGroup201JSONResponse(*apiGroup), nil
}

func (c *APIController) DeleteGroupByID(
	ctx context.Context,
	request cmkapi.DeleteGroupByIDRequestObject,
) (cmkapi.DeleteGroupByIDResponseObject, error) {
	err := c.Manager.Group.DeleteGroupByID(ctx, request.GroupID)
	if err != nil {
		return nil, err
	}

	return cmkapi.DeleteGroupByID204Response(struct{}{}), nil
}

func (c *APIController) GetGroupByID(
	ctx context.Context,
	request cmkapi.GetGroupByIDRequestObject,
) (cmkapi.GetGroupByIDResponseObject, error) {
	group, err := c.Manager.Group.GetGroupByID(ctx, request.GroupID)
	if err != nil {
		return nil, err
	}

	apiGroup, err := tfGroup.ToAPI(*group)
	if err != nil {
		return nil, err
	}

	return cmkapi.GetGroupByID200JSONResponse(*apiGroup), nil
}

func (c *APIController) UpdateGroup(
	ctx context.Context,
	request cmkapi.UpdateGroupRequestObject,
) (cmkapi.UpdateGroupResponseObject, error) {
	group, err := c.Manager.Group.UpdateGroup(ctx, request.GroupID, *request.Body)
	if err != nil {
		return nil, err
	}

	apiGroup, err := tfGroup.ToAPI(*group)
	if err != nil {
		return nil, err
	}

	return cmkapi.UpdateGroup200JSONResponse(*apiGroup), nil
}

func (c *APIController) CheckGroupsIAM(
	ctx context.Context,
	request cmkapi.CheckGroupsIAMRequestObject,
) (cmkapi.CheckGroupsIAMResponseObject, error) {
	result, err := c.Manager.Group.CheckIAMExistenceOfGroups(ctx, request.Body.IamIdentifiers)
	if err != nil {
		return nil, err
	}

	responseValues := make([]cmkapi.GroupIAMExistence, len(result))
	for i, v := range result {
		responseValues[i] = cmkapi.GroupIAMExistence{
			IamIdentifier: &v.IAMIdentifier,
			Exists:        v.Exists,
		}
	}

	return cmkapi.CheckGroupsIAM200JSONResponse{
		Value: responseValues,
	}, nil
}
