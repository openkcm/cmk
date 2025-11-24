package cmk

import (
	"context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/system"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/utils/odata"
	"github.com/openkcm/cmk/utils/ptr"
)

var getSystemsSchema odata.FilterSchema = odata.FilterSchema{
	Entries: []odata.FilterSchemaEntry{
		{
			FilterName: "keyConfigurationID",
			FilterType: odata.UUID,
			DBName:     repo.KeyConfigIDField,
		},
		{
			FilterName:    "region",
			FilterType:    odata.String,
			DBName:        repo.RegionField,
			ValueModifier: odata.ToUpper,
		},
		{
			FilterName:    "type",
			FilterType:    odata.String,
			DBName:        repo.TypeField,
			ValueModifier: odata.ToUpper,
		},
	},
}

func (c *APIController) GetAllSystems(ctx context.Context,
	request cmkapi.GetAllSystemsRequestObject,
) (cmkapi.GetAllSystemsResponseObject, error) {
	refreshed := c.Manager.System.RefreshSystemsData(ctx)

	queryMapper := odata.NewQueryOdataMapper(getSystemsSchema)

	err := queryMapper.ParseFilter(request.Params.Filter)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrBadOdataFilter, err)
	}

	queryMapper.SetPaging(request.Params.Skip, request.Params.Top)

	systems, total, err := c.Manager.System.GetAllSystems(ctx, queryMapper)
	if err != nil {
		return nil, err
	}

	values := make([]cmkapi.System, len(systems))
	for i, sys := range systems {
		apiSys, err := system.ToAPI(*sys, &c.config.ContextModels.System)
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

	systemResponse, err := system.ToAPI(*sys, &c.config.ContextModels.System)
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
	if c.Manager.Workflow.IsWorkflowEnabled(ctx) {
		return nil, apierrors.ErrActionRequireWorkflow
	}

	dbSystem, err := c.Manager.System.PatchSystemLinkByID(ctx, request.SystemID, *request.Body)
	if err != nil {
		return nil, err
	}

	systemResponse, err := system.ToAPI(*dbSystem, &c.config.ContextModels.System)
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
	if c.Manager.Workflow.IsWorkflowEnabled(ctx) {
		return nil, apierrors.ErrActionRequireWorkflow
	}

	err := c.Manager.System.DeleteSystemLinkByID(ctx, request.SystemID)
	if err != nil {
		return nil, err
	}

	return cmkapi.DeleteSystemLinkByID204Response(struct{}{}), nil
}
