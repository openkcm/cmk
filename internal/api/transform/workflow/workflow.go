package workflow

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	groupTransform "github.com/openkcm/cmk/internal/api/transform/group"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	wfMechanism "github.com/openkcm/cmk/internal/workflow"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
	"github.com/openkcm/cmk/utils/sanitise"
)

var ErrExpiryGreaterThanMaximum = errors.New("expiry exceeds maximum")

const (
	// AdditionalInfoMessageInsufficientApprovers is the message for the insufficient approvers warning
	AdditionalInfoMessageInsufficientApprovers = "The number of eligible approvers is currently" +
		" insufficient to meet the minimum approval criteria."
	// AdditionalInfoMessageEligibilityCheckError is the message when eligibility verification fails
	AdditionalInfoMessageEligibilityCheckError = "Unable to verify workflow eligibility. " +
		"The approval system may be temporarily unavailable. Please try again later or contact support."
	// AdditionalInfoMessageInitiatorIneligible is the message when initiator is no longer eligible to confirm
	AdditionalInfoMessageInitiatorIneligible = "The workflow initiator is no longer eligible to confirm this workflow."
)

// buildEligibilityAdditionalInfo creates additional info items based on eligibility status
func buildEligibilityAdditionalInfo(
	eligibility *manager.WorkflowEligibility,
	eligibilityErr error,
) *[]cmkapi.WorkflowAdditionalInfo {
	var apiInfoItems []cmkapi.WorkflowAdditionalInfo

	// If eligibility check failed, show only the error (takes precedence over warnings)
	if eligibilityErr != nil {
		apiInfoItems = append(apiInfoItems, cmkapi.WorkflowAdditionalInfo{
			Code:     cmkapi.WorkflowAdditionalInfoCodeWORKFLOWELIGIBILITYCHECKFAILED,
			Severity: cmkapi.WorkflowAdditionalInfoSeverityERROR,
			Message:  AdditionalInfoMessageEligibilityCheckError,
		})
	} else if eligibility != nil {
		// No error - show all applicable warnings
		if eligibility.InitiatorIneligible {
			apiInfoItems = append(apiInfoItems, cmkapi.WorkflowAdditionalInfo{
				Code:     cmkapi.WorkflowAdditionalInfoCodeINITIATORINELIGIBLE,
				Severity: cmkapi.WorkflowAdditionalInfoSeverityWARNING,
				Message:  AdditionalInfoMessageInitiatorIneligible,
			})
		}
		if eligibility.InsufficientApprovers {
			apiInfoItems = append(apiInfoItems, cmkapi.WorkflowAdditionalInfo{
				Code:     cmkapi.WorkflowAdditionalInfoCodeINSUFFICIENTAPPROVERS,
				Severity: cmkapi.WorkflowAdditionalInfoSeverityWARNING,
				Message:  AdditionalInfoMessageInsufficientApprovers,
			})
		}
	}

	if len(apiInfoItems) > 0 {
		return &apiInfoItems
	}
	return nil
}

// ToAPIOpt is a functional option for customizing the ToAPI transformation.
type ToAPIOpt func(*cmkapi.Workflow) error

// WithDetailed enriches the workflow with detailed information (approvers, groups, transitions, summary).
func WithDetailed(
	ctx context.Context,
	approvers []*model.WorkflowApprover,
	idm identitymanagement.IdentityManagement,
	approverGroups []*model.Group,
	transitions []wfMechanism.Transition,
	approvalSummary *wfMechanism.ApprovalSummary,
) ToAPIOpt {
	return func(w *cmkapi.Workflow) error {
		if approvers != nil {
			decisions := make([]cmkapi.WorkflowApprover, 0, len(approvers))
			for _, approver := range approvers {
				apiApprover, err := ApproverToAPI(ctx, *approver, idm)
				if err != nil {
					return err
				}
				decisions = append(decisions, apiApprover)
			}
			w.Decisions = &decisions
		}

		if approverGroups != nil {
			apiApproverGroups := make([]cmkapi.Group, 0, len(approverGroups))
			for _, group := range approverGroups {
				apiGroup, err := groupTransform.ToAPI(*group)
				if err != nil {
					return err
				}
				apiApproverGroups = append(apiApproverGroups, *apiGroup)
			}
			w.ApproverGroups = &apiApproverGroups
		}

		if transitions != nil {
			availableTransitions := make([]cmkapi.WorkflowTransitionValue, 0, len(transitions))
			for _, transition := range transitions {
				apiTransition := cmkapi.WorkflowTransitionValue(transition)
				availableTransitions = append(availableTransitions, apiTransition)
			}
			w.AvailableTransitions = &availableTransitions
		}

		if approvalSummary != nil {
			w.ApprovalSummary = &cmkapi.WorkflowApprovalSummary{
				Approved:    ptr.PointTo(approvalSummary.Approvals),
				Rejected:    ptr.PointTo(approvalSummary.Rejections),
				Pending:     ptr.PointTo(approvalSummary.Pending),
				TargetScore: ptr.PointTo(approvalSummary.TargetScore),
			}
		}

		return nil
	}
}

// ToAPI converts a workflow model to an API workflow presentation.
// eligibility contains eligibility check results (insufficient approvers, initiator ineligible).
// eligibilityErr should be passed if there was an error checking approver eligibility.
func ToAPI(
	ctx context.Context,
	w model.Workflow,
	eligibility *manager.WorkflowEligibility,
	eligibilityErr error,
	identityManager identitymanagement.IdentityManagement,
	opts ...ToAPIOpt,
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

	// Build metadata with additional info
	metadata := &cmkapi.WorkflowMetadata{
		CreatedAt:      ptr.PointTo(w.CreatedAt),
		UpdatedAt:      ptr.PointTo(w.UpdatedAt),
		AdditionalInfo: buildEligibilityAdditionalInfo(eligibility, eligibilityErr),
	}

	base := &cmkapi.Workflow{
		Id:                     ptr.PointTo(w.ID),
		InitiatorID:            w.InitiatorID,
		InitiatorName:          initiatorName,
		State:                  cmkapi.WorkflowState(strings.ToUpper(w.State.String())),
		ActionType:             cmkapi.WorkflowActionType(strings.ToUpper(w.ActionType.String())),
		ArtifactType:           cmkapi.WorkflowArtifactType(strings.ToUpper(w.ArtifactType.String())),
		ArtifactName:           w.ArtifactName,
		ParametersResourceName: w.ParametersResourceName,
		ParametersResourceType: parametersResourceType,
		ArtifactID:             w.ArtifactID,
		Parameters:             ptr.PointTo(w.Parameters),
		FailureReason:          ptr.PointTo(w.FailureReason),
		Metadata:               metadata,
		ExpiresAt:              w.ExpiryDate,
	}

	// Apply optional transformations
	for _, opt := range opts {
		err := opt(base)
		if err != nil {
			return nil, err
		}
	}

	return base, nil
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

	businessUserData, err := cmkcontext.ExtractBusinessUserData(ctx)
	if err != nil {
		return nil, err
	}

	wf := &model.Workflow{
		ID:           uuid.New(),
		ActionType:   model.WorkflowActionType(strings.ToUpper(string(apiWorkflow.ActionType))),
		ArtifactType: model.WorkflowArtifactType(strings.ToUpper(string(apiWorkflow.ArtifactType))),
		ArtifactID:   apiWorkflow.ArtifactID,
		InitiatorID:  businessUserData.Identifier,
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
