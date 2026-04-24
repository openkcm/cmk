package cmk

import (
	"context"
	"slices"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/system"
	wfWorkflow "github.com/openkcm/cmk/internal/api/transform/workflow"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/workflow"
	"github.com/openkcm/cmk/utils/odata"
	"github.com/openkcm/cmk/utils/ptr"
)

var getSystemsSchema = odata.FilterSchema{
	Entries: []odata.FilterSchemaEntry{
		{
			FilterName: "keyConfigurationID",
			FilterType: odata.UUID,
			DBName:     repo.KeyConfigIDField,
		},
		{
			FilterName:     "region",
			FilterType:     odata.String,
			DBName:         repo.RegionField,
			ValueValidator: odata.MaxLengthValidator(constants.QueryMaxLengthSystem),
		},
		{
			FilterName:     "type",
			FilterType:     odata.String,
			DBName:         repo.TypeField,
			ValueModifier:  odata.ToUpper,
			ValueValidator: odata.MaxLengthValidator(constants.QueryMaxLengthSystem),
		},
		{
			FilterName:     "status",
			FilterType:     odata.String,
			DBName:         repo.StatusField,
			ValueModifier:  odata.ToUpper,
			ValueValidator: odata.MaxLengthValidator(constants.QueryMaxLengthSystem),
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

	queryMapper.SetPaging(request.Params.Skip, request.Params.Top, request.Params.Count)

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

	var systemResponse *cmkapi.System
	if sys.Status == cmkapi.SystemStatusUNDERWORKFLOW {
		systemResponse, err = c.handleSystemUnderWorkflow(ctx, sys)
	} else {
		systemResponse, err = system.ToAPI(*sys, &c.config.ContextModels.System)
	}

	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformSystemToAPI, err)
	}

	return cmkapi.GetSystemByID200JSONResponse(*systemResponse), nil
}

func (c *APIController) LinkSystemAction(
	ctx context.Context,
	request cmkapi.LinkSystemActionRequestObject,
) (cmkapi.LinkSystemActionResponseObject, error) {
	required, err := c.Manager.Workflow.IsWorkflowRequired(ctx)
	if err != nil {
		return nil, err
	}

	if required {
		return nil, apierrors.ErrActionRequireWorkflow
	}

	dbSystem, err := c.Manager.System.LinkSystemAction(ctx, request.SystemID, *request.Body)
	if err != nil {
		return nil, err
	}

	systemResponse, err := system.ToAPI(*dbSystem, &c.config.ContextModels.System)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformSystemToAPI, err)
	}

	return cmkapi.LinkSystemAction200JSONResponse(*systemResponse), nil
}

func (c *APIController) GetRecoveryActions(
	ctx context.Context,
	request cmkapi.GetRecoveryActionsRequestObject,
) (cmkapi.GetRecoveryActionsResponseObject, error) {
	actions, err := c.Manager.System.GetRecoveryActions(ctx, request.SystemID)
	if err != nil {
		return nil, err
	}

	return cmkapi.GetRecoveryActions200JSONResponse(actions), nil
}

func (c *APIController) SendRecoveryActions(
	ctx context.Context,
	request cmkapi.SendRecoveryActionsRequestObject,
) (cmkapi.SendRecoveryActionsResponseObject, error) {
	err := c.Manager.System.SendRecoveryActions(ctx, request.SystemID, request.Body.Action)
	if err != nil {
		return nil, err
	}

	return cmkapi.SendRecoveryActions200Response{}, nil
}

func (c *APIController) UnlinkSystemAction(
	ctx context.Context,
	request cmkapi.UnlinkSystemActionRequestObject,
) (cmkapi.UnlinkSystemActionResponseObject, error) {
	required, err := c.Manager.Workflow.IsWorkflowRequired(ctx)
	if err != nil {
		return nil, err
	}

	if required {
		return nil, apierrors.ErrActionRequireWorkflow
	}

	err = c.Manager.System.UnlinkSystemAction(ctx, request.SystemID, "")
	if err != nil {
		return nil, err
	}

	return cmkapi.UnlinkSystemAction204Response(struct{}{}), nil
}

func (c *APIController) handleSystemUnderWorkflow(
	ctx context.Context,
	sys *model.System,
) (*cmkapi.System, error) {
	workflows, _, err := c.Manager.Workflow.GetWorkflows(ctx, manager.WorkflowFilter{
		ArtifactType: workflow.ArtifactTypeSystem.String(),
		ArtifactID:   sys.ID,
	})
	if err != nil || len(workflows) < 1 {
		return nil, errs.Wrapf(err, "error finding workflow")
	}
	wf := workflows[0]

	approvers, _, err := c.Manager.Workflow.ListWorkflowApprovers(ctx, wf.ID, true, repo.Pagination{})
	if err != nil {
		return nil, err
	}
	approverGroups, err := c.Manager.Workflow.GetWorkflowApproverGroups(ctx, wf)
	if err != nil {
		return nil, err
	}

	user, err := c.Manager.User.GetUserInfo(ctx)
	if err != nil {
		return nil, err
	}

	isApprover := slices.ContainsFunc(approvers, func(e *model.WorkflowApprover) bool {
		return e.UserID == user.Identifier
	})

	if user.Identifier != wf.InitiatorID && !isApprover {
		return system.ToAPI(
			*sys,
			&c.config.ContextModels.System,
			system.WithWorkflow(wf, wfWorkflow.WithDetailed(nil, approverGroups, nil, nil)),
		)
	}

	transitions, err := c.Manager.Workflow.GetWorkflowAvailableTransitions(ctx, wf)
	if err != nil {
		return nil, err
	}

	approvalSummary, err := c.Manager.Workflow.GetWorkflowApprovalSummary(ctx, wf)
	if err != nil {
		return nil, err
	}

	return system.ToAPI(
		*sys,
		&c.config.ContextModels.System,
		system.WithWorkflow(wf, wfWorkflow.WithDetailed(approvers, approverGroups, transitions, approvalSummary)),
	)
}
