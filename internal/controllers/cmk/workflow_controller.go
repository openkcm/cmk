package cmk

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	wfTransform "github.com/openkcm/cmk/internal/api/transform/workflow"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	wfMechanism "github.com/openkcm/cmk/internal/workflow"
	"github.com/openkcm/cmk/utils/odata"
	"github.com/openkcm/cmk/utils/ptr"
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
			FilterName:     "artifactName",
			FilterType:     odata.String,
			DBName:         repo.ArtifactNameField,
			ValueValidator: odata.MaxLengthValidator(constants.QueryMaxLengthName),
		},
		{
			FilterName:     "parametersResourceName",
			FilterType:     odata.String,
			DBName:         repo.ParamResourceNameField,
			ValueValidator: odata.MaxLengthValidator(constants.QueryMaxLengthName),
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
	workflow, err := c.Manager.Workflow.GetWorkflowByID(ctx, request.WorkflowID)
	if err != nil {
		return nil, err
	}

	// Expand approvers
	approvers, _, err := c.Manager.Workflow.ListWorkflowApprovers(ctx, request.WorkflowID, true, 0, 0)
	if err != nil {
		return nil, err
	}

	// Expand approver groups
	approverGroups, err := c.getApproverGroups(ctx, workflow)
	if err != nil {
		return nil, err
	}

	// Expand available transitions
	transitions, err := c.Manager.Workflow.GetWorkflowAvailableTransitions(ctx, workflow)
	if err != nil {
		return nil, err
	}

	// Expand approval summary
	approvalSummary, err := c.Manager.Workflow.GetWorkflowApprovalSummary(ctx, workflow)
	if err != nil {
		return nil, err
	}

	apiWorkflow, err := wfTransform.ToAPIDetailed(*workflow, approvers, approverGroups, transitions, approvalSummary)
	if err != nil {
		return nil, err
	}

	return cmkapi.GetWorkflowByID200JSONResponse(*apiWorkflow), nil
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

func (c *APIController) getApproverGroups(
	ctx context.Context,
	workflow *model.Workflow,
) ([]*model.Group, error) {
	var (
		IDs []uuid.UUID
	)

	if workflow.ApproverGroupIDs == nil {
		return []*model.Group{}, nil
	}

	err := json.Unmarshal(workflow.ApproverGroupIDs, &IDs)
	if err != nil {
		return nil, err
	}

	groups := make([]*model.Group, 0, len(IDs))
	for _, id := range IDs {
		group, err := c.Manager.Group.GetGroupByID(ctx, id)
		if err != nil {
			log.Warn(ctx, "failed to expand workflow approver group", slog.Any("error", err))

			// Return a placeholder group if the group cannot be found. We can still make use of the ID.
			groups = append(groups, &model.Group{
				ID:   id,
				Name: "NOT_AVAILABLE",
				Role: constants.KeyAdminRole,
			})
			continue
		}

		groups = append(groups, group)
	}

	return groups, nil
}
