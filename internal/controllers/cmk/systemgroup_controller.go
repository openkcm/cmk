package cmk

import (
	"context"

	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/api/cmk/transform/systemgroup"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/utils/ptr"
)

func (c *APIController) GetAllSystemGroups(
	ctx context.Context,
	request cmkapi.GetAllSystemGroupsRequestObject,
) (cmkapi.GetAllSystemGroupsResponseObject, error) {
	pagination := repo.Pagination{
		Skip:  ptr.GetPtrOrDefault(request.Params.Skip, constants.DefaultSkip),
		Top:   ptr.GetPtrOrDefault(request.Params.Top, constants.DefaultTop),
		Count: ptr.GetSafeDeref(request.Params.Count),
	}

	expand := request.Params.Expand != nil && request.Params.Expand.Valid()
	systemGroups, total, err := c.Manager.SystemGroup.GetAllSystemGroups(ctx, manager.SystemGroupFilter{
		ExpandSystems: expand,
		Pagination:    pagination,
	})
	if err != nil {
		return nil, err
	}

	values := make([]cmkapi.SystemGroup, len(systemGroups))

	for i, sg := range systemGroups {
		val, err := systemgroup.ToAPI(sg)
		if err != nil {
			return nil, apierrors.ErrTransformSystemGroupsToAPI
		}

		values[i] = *val
	}

	response := cmkapi.SystemGroupList{
		Value: values,
	}

	if pagination.Count {
		response.Count = new(total)
	}
	return cmkapi.GetAllSystemGroups200JSONResponse(response), nil
}

func (c *APIController) UpdateSystemGroup(
	ctx context.Context,
	request cmkapi.UpdateSystemGroupRequestObject,
) (cmkapi.UpdateSystemGroupResponseObject, error) {
	sg, err := c.Manager.SystemGroup.UpdateSystemGroup(ctx, request.SystemGroupID, *request.Body)
	if err != nil {
		return nil, err
	}

	val, err := systemgroup.ToAPI(sg)
	if err != nil {
		return nil, apierrors.ErrTransformSystemGroupsToAPI
	}
	return cmkapi.UpdateSystemGroup200JSONResponse(*val), nil
}
