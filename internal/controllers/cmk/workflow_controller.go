package cmk

import (
	"context"
	"slices"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	wfTransform "github.tools.sap/kms/cmk/internal/api/transform/workflow"
	"github.tools.sap/kms/cmk/internal/apierrors"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/internal/repo"
	wfMechanism "github.tools.sap/kms/cmk/internal/workflow"
	"github.tools.sap/kms/cmk/utils/odata"
	"github.tools.sap/kms/cmk/utils/ptr"
)

func (c *APIController) CheckWorkflow(
	ctx context.Context,
	request cmkapi.CheckWorkflowRequestObject,
) (cmkapi.CheckWorkflowResponseObject, error) {
	workflowConfig, err := c.Manager.Workflow.WorkflowConfig(ctx)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformWorkflowFromAPI, err)
	}

	workflow, err := wfTransform.FromAPI(ctx, *request.Body,
		workflowConfig.DefaultExpiryPeriodDays, workflowConfig.MaxExpiryPeriodDays)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformWorkflowFromAPI, err)
	}

	status, err := c.Manager.Workflow.CheckWorkflow(ctx, workflow)
	if err != nil {
		return nil, err
	}

	response := cmkapi.CheckWorkflow200JSONResponse{
		Exists:   &status.Exists,
		Required: &status.Enabled,
	}

	return response, nil
}

var getWorkflowsSchema = odata.FilterSchema{
	Entries: []odata.FilterSchemaEntry{
		{
			FilterName: "userID",
			FilterType: odata.String,
			DBName:     repo.UserIDField,
			DBQuery:    odata.NoQuery, // Manager handles this case
		},
		{
			FilterName: "artifactId",
			FilterType: odata.UUID,
			DBName:     repo.ArtifactIDField,
		},
		{
			FilterName: "artifactType",
			FilterType: odata.String,
			DBName:     repo.ArtifactTypeField,
			ValueValidator: func(s string) bool {
				return slices.Contains(wfMechanism.ArtifactTypes,
					wfMechanism.ArtifactType(s))
			},
			ValueModifier: odata.ToUpper,
		},
		{
			FilterName: "actionType",
			FilterType: odata.String,
			DBName:     repo.ActionTypeField,
			ValueValidator: func(s string) bool {
				return slices.Contains(wfMechanism.ActionTypes,
					wfMechanism.ActionType(s))
			},
			ValueModifier: odata.ToUpper,
		},
		{
			FilterName: "state",
			FilterType: odata.String,
			DBName:     repo.StateField,
			ValueValidator: func(s string) bool {
				return slices.Contains(wfMechanism.States,
					wfMechanism.State(s))
			},
			ValueModifier: odata.ToUpper,
		},
	},
}

// GetWorkflows returns a list of workflows
func (c *APIController) GetWorkflows(
	ctx context.Context,
	request cmkapi.GetWorkflowsRequestObject,
) (cmkapi.GetWorkflowsResponseObject, error) {
	odataQueryMapper := odata.NewQueryOdataMapper(getWorkflowsSchema)

	err := odataQueryMapper.ParseFilter(request.Params.Filter)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrBadOdataFilter, err)
	}

	odataQueryMapper.SetPaging(request.Params.Skip, request.Params.Top)

	workflowQueryMapper, err := manager.NewWorkflowFilterFromOData(*odataQueryMapper)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrBadOdataFilter, err)
	}

	workflows, count, err := c.Manager.Workflow.GetWorkflows(ctx, workflowQueryMapper)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrGetWorkflow, err)
	}

	values := make([]cmkapi.Workflow, len(workflows))

	for i, dbWorkflow := range workflows {
		apiWorkflow, err := wfTransform.ToAPI(*dbWorkflow)
		if err != nil {
			return nil, errs.Wrap(apierrors.ErrGetWorkflow, err)
		}

		values[i] = *apiWorkflow
	}

	response := cmkapi.GetWorkflows200JSONResponse{
		Value: values,
	}
	if ptr.GetSafeDeref(request.Params.Count) {
		response.Count = &count
	}

	return response, nil
}

// CreateWorkflow creates a new workflow
func (c *APIController) CreateWorkflow(ctx context.Context,
	request cmkapi.CreateWorkflowRequestObject,
) (cmkapi.CreateWorkflowResponseObject, error) {
	workflowConfig, err := c.Manager.Workflow.WorkflowConfig(ctx)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformWorkflowFromAPI, err)
	}

	workflow, err := wfTransform.FromAPI(ctx, *request.Body,
		workflowConfig.DefaultExpiryPeriodDays, workflowConfig.MaxExpiryPeriodDays)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformWorkflowFromAPI, err)
	}

	workflow, err = c.Manager.Workflow.CreateWorkflow(ctx, workflow)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrCreateWorkflow, err)
	}

	returnAPIWorkflow, err := wfTransform.ToAPI(*workflow)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformWorkflowToAPI, err)
	}

	return cmkapi.CreateWorkflow201JSONResponse(*returnAPIWorkflow), nil
}

func (c *APIController) GetWorkflowByID(ctx context.Context,
	request cmkapi.GetWorkflowByIDRequestObject,
) (cmkapi.GetWorkflowByIDResponseObject, error) {
	workflow, err := c.Manager.Workflow.GetWorkflowsByID(ctx, request.WorkflowID)
	if err != nil {
		return nil, err
	}

	apiWorkflow, err := wfTransform.ToAPI(*workflow)
	if err != nil {
		return nil, err
	}

	return cmkapi.GetWorkflowByID201JSONResponse(*apiWorkflow), nil
}

// ListWorkflowApproversByWorkflowID updates a workflow by ID
func (c *APIController) ListWorkflowApproversByWorkflowID(
	ctx context.Context,
	request cmkapi.ListWorkflowApproversByWorkflowIDRequestObject,
) (cmkapi.ListWorkflowApproversByWorkflowIDResponseObject, error) {
	skip := ptr.GetIntOrDefault(request.Params.Skip, constants.DefaultSkip)
	top := ptr.GetIntOrDefault(request.Params.Top, constants.DefaultTop)

	approvers, count, err := c.Manager.Workflow.ListWorkflowApprovers(ctx, request.WorkflowID, skip, top)
	if err != nil {
		return nil, err
	}

	// Convert each Approver to its response format
	values := make([]cmkapi.WorkflowApprover, len(approvers))

	for i, approver := range approvers {
		value, err := wfTransform.ApproverToAPI(*approver)
		if err != nil {
			return nil, err
		}

		values[i] = value
	}

	response := cmkapi.ListWorkflowApproversByWorkflowID200JSONResponse{
		Value: values,
	}

	if ptr.GetSafeDeref(request.Params.Count) {
		response.Count = ptr.PointTo(count)
	}

	return response, nil
}

// TransitionWorkflow executes a transition on a workflow by ID

func (c *APIController) TransitionWorkflow(
	ctx context.Context,
	request cmkapi.TransitionWorkflowRequestObject,
) (cmkapi.TransitionWorkflowResponseObject, error) {
	transitionBody := request.Body

	transition := wfMechanism.Transition(transitionBody.Transition)

	workflow, err := c.Manager.Workflow.TransitionWorkflow(ctx, request.WorkflowID, transition)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrWorkflowCannotTransition, err)
	}

	apiWorkflow, err := wfTransform.ToAPI(*workflow)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformWorkflowToAPI, err)
	}

	return cmkapi.TransitionWorkflow200JSONResponse(*apiWorkflow), nil
}
