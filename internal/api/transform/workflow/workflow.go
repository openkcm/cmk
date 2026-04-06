package workflow

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	groupTransform "github.com/openkcm/cmk/internal/api/transform/group"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	wfMechanism "github.com/openkcm/cmk/internal/workflow"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
	"github.com/openkcm/cmk/utils/sanitise"
)

var ErrExpiryGreaterThanMaximum = errors.New("expiry exceeds maximum")

// ToAPI converts a workflow model to an API workflow presentation.
func ToAPI(
	ctx context.Context,
	w model.Workflow,
	identityManager identitymanagement.IdentityManagement,
) (*cmkapi.Workflow, error) {
	err := sanitise.Sanitize(&w)
	if err != nil {
		return nil, err
	}

	var parametersResourceType *cmkapi.WorkflowParametersResourceType
	if w.ParametersResourceType != nil {
		resourceType := cmkapi.WorkflowParametersResourceType(strings.ToUpper(*w.ParametersResourceType))
		parametersResourceType = &resourceType
	}

	initiatorName, err := w.GetInitiatorName(ctx, identityManager)
	if err != nil {
		return nil, err
	}

	return &cmkapi.Workflow{
		Id:                     ptr.PointTo(w.ID),
		InitiatorID:            w.InitiatorID,
		InitiatorName:          initiatorName,
		State:                  cmkapi.WorkflowState(strings.ToUpper(w.State)),
		ActionType:             cmkapi.WorkflowActionType(strings.ToUpper(w.ActionType)),
		ArtifactType:           cmkapi.WorkflowArtifactType(strings.ToUpper(w.ArtifactType)),
		ArtifactName:           w.ArtifactName,
		ParametersResourceName: w.ParametersResourceName,
		ParametersResourceType: parametersResourceType,
		ArtifactID:             w.ArtifactID,
		Parameters:             ptr.PointTo(w.Parameters),
		FailureReason:          ptr.PointTo(w.FailureReason),
		Metadata: ptr.PointTo(cmkapi.WorkflowMetadata{
			CreatedAt: ptr.PointTo(w.CreatedAt),
			UpdatedAt: ptr.PointTo(w.UpdatedAt),
		}),
		ExpiresAt: w.ExpiryDate,
	}, nil
}

//nolint:funlen
func ToAPIDetailed(
	ctx context.Context,
	w model.Workflow,
	approvers []*model.WorkflowApprover,
	approverGroups []*model.Group,
	transitions []wfMechanism.Transition,
	approvalSummary *wfMechanism.ApprovalSummary,
	identityManager identitymanagement.IdentityManagement,
) (*cmkapi.DetailedWorkflow, error) {
	base, err := ToAPI(ctx, w, identityManager)
	if err != nil {
		return nil, err
	}

	detailed := &cmkapi.DetailedWorkflow{
		Id:                     base.Id,
		InitiatorID:            base.InitiatorID,
		InitiatorName:          base.InitiatorName,
		State:                  base.State,
		ActionType:             base.ActionType,
		ArtifactType:           base.ArtifactType,
		ArtifactName:           base.ArtifactName,
		ArtifactID:             base.ArtifactID,
		Parameters:             base.Parameters,
		ParametersResourceName: base.ParametersResourceName,
		ParametersResourceType: base.ParametersResourceType,
		FailureReason:          base.FailureReason,
		Metadata:               base.Metadata,
		ExpiresAt:              base.ExpiresAt,
		ApprovalSummary: &cmkapi.WorkflowApprovalSummary{
			Approved:    ptr.PointTo(approvalSummary.Approvals),
			Rejected:    ptr.PointTo(approvalSummary.Rejections),
			Pending:     ptr.PointTo(approvalSummary.Pending),
			TargetScore: ptr.PointTo(approvalSummary.TargetScore),
		},
	}

	decisions := make([]cmkapi.WorkflowApprover, 0, len(approvers))
	for _, approver := range approvers {
		apiApprover, err := ApproverToAPI(ctx, *approver, identityManager)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, apiApprover)
	}
	detailed.Decisions = decisions

	apiApproverGroups := make([]cmkapi.Group, 0, len(approverGroups))
	for _, group := range approverGroups {
		apiGroup, err := groupTransform.ToAPI(*group)
		if err != nil {
			return nil, err
		}
		apiApproverGroups = append(apiApproverGroups, *apiGroup)
	}
	detailed.ApproverGroups = apiApproverGroups

	availableTransitions := make([]cmkapi.WorkflowTransitionValue, 0, len(transitions))
	for _, transition := range transitions {
		apiTransition := cmkapi.WorkflowTransitionValue(transition)
		availableTransitions = append(availableTransitions, apiTransition)
	}
	detailed.AvailableTransitions = availableTransitions

	return detailed, nil
}

// FromAPI converts an API workflow presentation to a workflow model.
func FromAPI(
	ctx context.Context,
	apiWorkflow cmkapi.WorkflowBody,
	defaultExpiryPeriod, maxExpiryPeriod int,
) (*model.Workflow, error) {
	defaultExpiryDate := time.Now().AddDate(0, 0, defaultExpiryPeriod)
	maxExpiryDate := time.Now().AddDate(0, 0, maxExpiryPeriod)

	expiryDate := defaultExpiryDate
	if apiWorkflow.ExpiresAt != nil {
		expiryDate = *apiWorkflow.ExpiresAt
	}

	if expiryDate.After(maxExpiryDate) {
		return nil, ErrExpiryGreaterThanMaximum
	}

	clientData, err := cmkcontext.ExtractClientData(ctx)
	if err != nil {
		return nil, err
	}

	wf := &model.Workflow{
		ID:           uuid.New(),
		ActionType:   strings.ToUpper(string(apiWorkflow.ActionType)),
		ArtifactType: strings.ToUpper(string(apiWorkflow.ArtifactType)),
		ArtifactID:   apiWorkflow.ArtifactID,
		InitiatorID:  clientData.Identifier,
		ExpiryDate:   &expiryDate,
	}

	if apiWorkflow.Parameters != nil {
		wf.Parameters = *apiWorkflow.Parameters
	}

	return wf, nil
}

// ApproverToAPI converts a workflow approver model to an API workflow approver presentation.
func ApproverToAPI(
	ctx context.Context,
	approver model.WorkflowApprover,
	iam identitymanagement.IdentityManagement,
) (cmkapi.WorkflowApprover, error) {
	err := sanitise.Sanitize(&approver)
	if err != nil {
		return cmkapi.WorkflowApprover{}, err
	}

	name, err := approver.GetUserName(ctx, iam)
	if err != nil {
		return cmkapi.WorkflowApprover{}, err
	}

	return cmkapi.WorkflowApprover{
		Id:   approver.UserID,
		Name: ptr.PointTo(name),
		Decision: func() cmkapi.WorkflowApproverDecision {
			if approver.Approved.Valid {
				if approver.Approved.Bool {
					return cmkapi.WorkflowApproverDecisionAPPROVED
				}

				return cmkapi.WorkflowApproverDecisionREJECTED
			}
			return cmkapi.WorkflowApproverDecisionPENDING
		}(),
	}, nil
}
