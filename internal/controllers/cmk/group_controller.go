package cmk

import (
	"context"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/api/transform"
	tfGroup "github.com/openkcm/cmk-core/internal/api/transform/group"
	"github.com/openkcm/cmk-core/internal/apierrors"
	"github.com/openkcm/cmk-core/internal/constants"
	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/internal/manager"
	"github.com/openkcm/cmk-core/internal/model"
	cmkcontext "github.com/openkcm/cmk-core/utils/context"
	"github.com/openkcm/cmk-core/utils/ptr"
)

func (c *APIController) GetGroups(ctx context.Context,
	request cmkapi.GetGroupsRequestObject,
) (cmkapi.GetGroupsResponseObject, error) {
	skip := ptr.GetIntOrDefault(request.Params.Skip, constants.DefaultSkip)
	top := ptr.GetIntOrDefault(request.Params.Top, constants.DefaultTop)

	// log.Warn(ctx, "test", "test", "test") // USE THIS ONE

	groups, total, err := c.Manager.Group.GetGroups(ctx, skip, top)
	if err != nil {
		return nil, err
	}

	values, err := transform.ToList(groups, func(group model.Group) (*cmkapi.Group, error) {
		return tfGroup.ToAPI(group), nil
	})

	response := cmkapi.GroupList{
		Value: values,
	}

	if ptr.GetSafeDeref(request.Params.Count) {
		response.Count = ptr.PointTo(total)
	}

	return cmkapi.GetGroups200JSONResponse(response), err
}

func (c *APIController) CreateGroup(ctx context.Context,
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

	apiGroup := tfGroup.ToAPI(*group)

	return cmkapi.CreateGroup201JSONResponse(*apiGroup), nil
}

func (c *APIController) DeleteGroupByID(ctx context.Context,
	request cmkapi.DeleteGroupByIDRequestObject,
) (cmkapi.DeleteGroupByIDResponseObject, error) {
	err := c.Manager.Group.DeleteGroupByID(ctx, request.GroupID)
	if err != nil {
		return nil, err
	}

	return cmkapi.DeleteGroupByID204Response(struct{}{}), nil
}

func (c *APIController) GetGroupByID(ctx context.Context,
	request cmkapi.GetGroupByIDRequestObject,
) (cmkapi.GetGroupByIDResponseObject, error) {
	group, err := c.Manager.Group.GetGroupByID(ctx, request.GroupID)
	if err != nil {
		return nil, err
	}

	apiGroup := tfGroup.ToAPI(*group)

	return cmkapi.GetGroupByID200JSONResponse(*apiGroup), nil
}

func (c *APIController) UpdateGroup(ctx context.Context,
	request cmkapi.UpdateGroupRequestObject,
) (cmkapi.UpdateGroupResponseObject, error) {
	group, err := c.Manager.Group.UpdateGroup(ctx, request.GroupID, *request.Body)
	if err != nil {
		return nil, err
	}

	apiGroup := tfGroup.ToAPI(*group)

	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformKeyToAPI, err)
	}

	return cmkapi.UpdateGroup200JSONResponse(*apiGroup), nil
}
