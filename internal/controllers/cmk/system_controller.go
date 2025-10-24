package cmk

import (
	"context"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/api/transform/system"
	"github.com/openkcm/cmk-core/internal/apierrors"
	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/utils/ptr"
)

func (c *APIController) GetAllSystems(ctx context.Context,
	request cmkapi.GetAllSystemsRequestObject,
) (cmkapi.GetAllSystemsResponseObject, error) {
	refreshed := c.Manager.System.RefreshSystemsData(ctx)
	filter := c.Manager.System.NewSystemFilter(request)

	systems, total, err := c.Manager.System.GetAllSystems(ctx, filter)
	if err != nil {
		return nil, err
	}

	values := make([]cmkapi.System, len(systems))
	for i, sys := range systems {
		apiSys, err := system.ToAPI(*sys, &c.config.System)
		if err != nil {
			return nil, errs.Wrap(apierrors.ErrTransformSystemList, err)
		}

		values[i] = *apiSys
	}

	response := cmkapi.GetAllSystems200JSONResponse{
		Value:                values,
		SystemsDataRefreshed: refreshed,
	}

	if ptr.GetSafeDeref(request.Params.Count) {
		response.Count = ptr.PointTo(total)
	}

	return response, nil
}

func (c *APIController) GetSystemByID(ctx context.Context,
	request cmkapi.GetSystemByIDRequestObject,
) (cmkapi.GetSystemByIDResponseObject, error) {
	sys, err := c.Manager.System.GetSystemByID(ctx, request.SystemID)
	if err != nil {
		return nil, err
	}

	systemResponse, err := system.ToAPI(*sys, &c.config.System)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformSystemToAPI, err)
	}

	return cmkapi.GetSystemByID200JSONResponse(*systemResponse), nil
}

func (c *APIController) GetSystemLinkByID(ctx context.Context,
	request cmkapi.GetSystemLinkByIDRequestObject,
) (cmkapi.GetSystemLinkByIDResponseObject, error) {
	keyConfigID, err := c.Manager.System.GetSystemLinkByID(ctx, request.SystemID)
	if err != nil {
		return nil, err
	}

	return cmkapi.GetSystemLinkByID200JSONResponse{KeyConfigurationID: keyConfigID}, nil
}

// PatchSystemLinkByID updates a system link by its ID.
func (c *APIController) PatchSystemLinkByID(
	ctx context.Context,
	request cmkapi.PatchSystemLinkByIDRequestObject,
) (cmkapi.PatchSystemLinkByIDResponseObject, error) {
	if c.isWorkflowEnabled() {
		return nil, apierrors.ErrActionRequireWorkflow
	}

	dbSystem, err := c.Manager.System.PatchSystemLinkByID(ctx, request.SystemID, system.FromAPIPatch(*request.Body))
	if err != nil {
		return nil, err
	}

	systemResponse, err := system.ToAPI(*dbSystem, &c.config.System)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformSystemToAPI, err)
	}

	return cmkapi.PatchSystemLinkByID200JSONResponse(*systemResponse), nil
}

// DeleteSystemLinkByID deletes a system link by its ID.
func (c *APIController) DeleteSystemLinkByID(
	ctx context.Context,
	request cmkapi.DeleteSystemLinkByIDRequestObject,
) (cmkapi.DeleteSystemLinkByIDResponseObject, error) {
	if c.isWorkflowEnabled() {
		return nil, apierrors.ErrActionRequireWorkflow
	}

	err := c.Manager.System.DeleteSystemLinkByID(ctx, request.SystemID)
	if err != nil {
		return nil, err
	}

	return cmkapi.DeleteSystemLinkByID204Response(struct{}{}), nil
}
