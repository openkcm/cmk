package workflow

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/model"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
	"github.tools.sap/kms/cmk/utils/ptr"
	"github.tools.sap/kms/cmk/utils/sanitise"
)

var (
	ErrExpiryGreaterThanMaximum = errors.New("expiry exceeds maximum")
)

// ToAPI converts a workflow model to an API workflow presentation.
func ToAPI(w model.Workflow) (*cmkapi.Workflow, error) {
	err := sanitise.Stringlikes(&w)
	if err != nil {
		return nil, err
	}

	return &cmkapi.Workflow{
		Id:            ptr.PointTo(w.ID),
		InitiatorID:   ptr.PointTo(w.InitiatorID),
		InitiatorName: ptr.PointTo(w.InitiatorName),
		State:         ptr.PointTo(cmkapi.WorkflowState(strings.ToUpper(w.State))),
		ActionType:    cmkapi.WorkflowActionType(strings.ToUpper(w.ActionType)),
		ArtifactType:  cmkapi.WorkflowArtifactType(strings.ToUpper(w.ArtifactType)),
		ArtifactID:    w.ArtifactID,
		Parameters:    ptr.PointTo(w.Parameters),
		FailureReason: ptr.PointTo(w.FailureReason),
		Metadata: ptr.PointTo(cmkapi.WorkflowMetadata{
			CreatedAt: ptr.PointTo(w.CreatedAt),
			UpdatedAt: ptr.PointTo(w.UpdatedAt),
		}),
		ExpiresAt: &w.ExpiryDate,
	}, nil
}

// FromAPI converts an API workflow presentation to a workflow model.
func FromAPI(ctx context.Context, apiWorkflow cmkapi.Workflow,
	defaultExpiryPeriod, maxExpiryPeriod int) (*model.Workflow, error) {
	defaultExpiryDate := time.Now().AddDate(0, 0, defaultExpiryPeriod)
	maxExpiryDate := time.Now().AddDate(0, 0, maxExpiryPeriod)

	var expiryDate = defaultExpiryDate
	if apiWorkflow.ExpiresAt != nil {
		expiryDate = *apiWorkflow.ExpiresAt
	}

	if expiryDate.After(maxExpiryDate) {
		return nil, ErrExpiryGreaterThanMaximum
	}

	if apiWorkflow.Id == nil {
		newUUID := uuid.New()
		apiWorkflow.Id = &newUUID
	}

	clientData, err := cmkcontext.ExtractClientData(ctx)
	if err != nil {
		return nil, err
	}

	wf := &model.Workflow{
		ID:            *apiWorkflow.Id,
		ActionType:    strings.ToUpper(string(apiWorkflow.ActionType)),
		ArtifactType:  strings.ToUpper(string(apiWorkflow.ArtifactType)),
		ArtifactID:    apiWorkflow.ArtifactID,
		InitiatorID:   clientData.Identifier,
		InitiatorName: clientData.Email,
		ExpiryDate:    expiryDate,
	}

	if apiWorkflow.Parameters != nil {
		wf.Parameters = *apiWorkflow.Parameters
	}

	return wf, nil
}

// ApproverToAPI converts a workflow approver model to an API workflow approver presentation.
func ApproverToAPI(approver model.WorkflowApprover) (cmkapi.WorkflowApprover, error) {
	err := sanitise.Stringlikes(&approver)
	if err != nil {
		return cmkapi.WorkflowApprover{}, err
	}

	return cmkapi.WorkflowApprover{
		Id:   approver.UserID,
		Name: ptr.PointTo(approver.UserName),
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
