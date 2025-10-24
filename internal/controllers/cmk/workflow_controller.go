package cmk

import (
	"context"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	wfTransform "github.com/openkcm/cmk-core/internal/api/transform/workflow"
	"github.com/openkcm/cmk-core/internal/apierrors"
	"github.com/openkcm/cmk-core/internal/constants"
	"github.com/openkcm/cmk-core/internal/errs"
	wfMechanism "github.com/openkcm/cmk-core/internal/workflow"
	"github.com/openkcm/cmk-core/utils/ptr"
)

// GetWorkflows returns a list of workflows
func (c *APIController) GetWorkflows(
	ctx context.Context,
	request cmkapi.GetWorkflowsRequestObject,
) (cmkapi.GetWorkflowsResponseObject, error) {
	filter := c.Manager.Workflow.NewWorkflowFilter(request)

	workflows, count, err := c.Manager.Workflow.GetWorkflows(ctx, filter)
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
	workflow, err := wfTransform.FromAPI(*request.Body, request.Params.UserID)
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
		values[i] = cmkapi.WorkflowApprover{
			Id: approver.WorkflowID,
		}
	}

	response := cmkapi.ListWorkflowApproversByWorkflowID200JSONResponse{
		Value: values,
	}

	if ptr.GetSafeDeref(request.Params.Count) {
		response.Count = ptr.PointTo(count)
	}

	return response, nil
}

// AddWorkflowApproversByWorkflowID adds approvers to a workflow by ID

func (c *APIController) AddWorkflowApproversByWorkflowID(
	ctx context.Context,
	request cmkapi.AddWorkflowApproversByWorkflowIDRequestObject,
) (cmkapi.AddWorkflowApproversByWorkflowIDResponseObject, error) {
	workflow, err := c.Manager.Workflow.AddWorkflowApprovers(
		ctx,
		request.WorkflowID,
		request.Params.UserID,
		request.Body.Approvers,
	)
	if err != nil {
		return nil, errs.Wrap(err, apierrors.ErrAddApprovers)
	}

	apiWorkflow, err := wfTransform.ToAPI(*workflow)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformWorkflowToAPI, err)
	}

	return cmkapi.AddWorkflowApproversByWorkflowID204JSONResponse(*apiWorkflow), nil
}

// TransitionWorkflow executes a transition on a workflow by ID

func (c *APIController) TransitionWorkflow(
	ctx context.Context,
	request cmkapi.TransitionWorkflowRequestObject,
) (cmkapi.TransitionWorkflowResponseObject, error) {
	transitionBody := request.Body

	transition := wfMechanism.Transition(transitionBody.Transition)

	workflow, err := c.Manager.Workflow.TransitionWorkflow(ctx, request.Params.UserID, request.WorkflowID, transition)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrWorkflowCannotTransition, err)
	}

	apiWorkflow, err := wfTransform.ToAPI(*workflow)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformWorkflowToAPI, err)
	}

	return cmkapi.TransitionWorkflow200JSONResponse(*apiWorkflow), nil
}
