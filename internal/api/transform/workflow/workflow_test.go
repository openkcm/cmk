package workflow_test

import (
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/workflow"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/utils/ptr"
)

func TestWorkflow_ToAPI(t *testing.T) {
	workflowMutator := testutils.NewMutator(func() model.Workflow {
		return model.Workflow{
			ID:           uuid.New(),
			InitiatorID:  uuid.New(),
			State:        "INITIAL",
			ActionType:   "UPDATE_STATE",
			ArtifactType: "KEY",
			ArtifactID:   uuid.New(),
			Parameters:   "ENABLED",
		}
	})

	tests := []struct {
		name                 string
		dbWorkflow           model.Workflow
		expectedState        cmkapi.WorkflowState
		expectedActionType   cmkapi.WorkflowActionType
		expectedArtifactType cmkapi.WorkflowArtifactType
		errorExpected        bool
	}{
		{
			name:                 "TestWorkflow_ToAPI_Valid",
			dbWorkflow:           workflowMutator(),
			expectedState:        cmkapi.WorkflowStateEnumINITIAL,
			expectedActionType:   cmkapi.WorkflowActionTypeEnumUPDATESTATE,
			expectedArtifactType: cmkapi.WorkflowArtifactTypeEnumKEY,
		},
		{
			name: "TestWorkflow_ToAPI_Lowercase",
			dbWorkflow: workflowMutator(func(w *model.Workflow) {
				w.State = "initial"
				w.ActionType = "update_state"
				w.ArtifactType = "key"
			}),
			expectedState:        cmkapi.WorkflowStateEnumINITIAL,
			expectedActionType:   cmkapi.WorkflowActionTypeEnumUPDATESTATE,
			expectedArtifactType: cmkapi.WorkflowArtifactTypeEnumKEY,
		},
		{
			name: "TestWorkflow_ToAPI_WithFailureReason",
			dbWorkflow: workflowMutator(func(w *model.Workflow) {
				w.State = "FAILED"
				w.FailureReason = "Failed to update state"
			}),
			expectedState:        cmkapi.WorkflowStateEnumFAILED,
			expectedActionType:   cmkapi.WorkflowActionTypeEnumUPDATESTATE,
			expectedArtifactType: cmkapi.WorkflowArtifactTypeEnumKEY,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiWorkflow, err := workflow.ToAPI(tt.dbWorkflow)

			if tt.errorExpected {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.dbWorkflow.ID, *apiWorkflow.Id)
			assert.Equal(t, tt.dbWorkflow.InitiatorID, *apiWorkflow.InitiatorID)
			assert.Equal(t, tt.expectedState, *apiWorkflow.State)
			assert.Equal(t, tt.expectedActionType, apiWorkflow.ActionType)
			assert.Equal(t, tt.expectedArtifactType, apiWorkflow.ArtifactType)
			assert.Equal(t, tt.dbWorkflow.ArtifactID, apiWorkflow.ArtifactID)
			assert.Equal(t, tt.dbWorkflow.Parameters, *apiWorkflow.Parameters)
			assert.Equal(t, tt.dbWorkflow.FailureReason, *apiWorkflow.FailureReason)
		})
	}
}

func TestWorkflow_FromAPI(t *testing.T) {
	apiWorkflowMutator := testutils.NewMutator(func() cmkapi.Workflow {
		return cmkapi.Workflow{
			Id:           ptr.PointTo(uuid.New()),
			ActionType:   cmkapi.WorkflowActionTypeEnumUPDATESTATE,
			ArtifactType: cmkapi.WorkflowArtifactTypeEnumKEY,
			ArtifactID:   uuid.New(),
			Parameters:   ptr.PointTo("ENABLED"),
		}
	})

	tests := []struct {
		name          string
		apiWorkflow   cmkapi.Workflow
		userID        uuid.UUID
		errorExpected bool
	}{
		{
			name:        "TestWorkflow_ToAPI_Valid",
			apiWorkflow: apiWorkflowMutator(),
			userID:      uuid.New(),
		},
		{
			name: "TestWorkflow_ToAPI_NilID",
			apiWorkflow: apiWorkflowMutator(func(w *cmkapi.Workflow) {
				w.Id = nil
			}),
			userID: uuid.New(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, err := workflow.FromAPI(tt.apiWorkflow, tt.userID)

			if tt.errorExpected {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.apiWorkflow.Id == nil {
				assert.NotNil(t, w.ID)
			} else {
				assert.Equal(t, *tt.apiWorkflow.Id, w.ID)
			}

			assert.Equal(t, string(tt.apiWorkflow.ActionType), w.ActionType)
			assert.Equal(t, string(tt.apiWorkflow.ArtifactType), w.ArtifactType)
			assert.Equal(t, tt.apiWorkflow.ArtifactID, w.ArtifactID)
			assert.Equal(t, tt.userID, w.InitiatorID)
			assert.Equal(t, *tt.apiWorkflow.Parameters, w.Parameters)
		})
	}
}

func TestWorkflow_ApproverToAPI(t *testing.T) {
	tests := []struct {
		name     string
		input    model.WorkflowApprover
		expected cmkapi.WorkflowApproverDecision
	}{
		{
			name: "Approved",
			input: model.WorkflowApprover{
				UserID:   uuid.New(),
				UserName: "User1",
				Approved: sql.NullBool{Bool: true, Valid: true},
			},
			expected: cmkapi.WorkflowApproverDecisionAPPROVED,
		},
		{
			name: "Rejected",
			input: model.WorkflowApprover{
				UserID:   uuid.New(),
				UserName: "User2",
				Approved: sql.NullBool{Bool: false, Valid: true},
			},
			expected: cmkapi.WorkflowApproverDecisionREJECTED,
		},
		{
			name: "Pending",
			input: model.WorkflowApprover{
				UserID:   uuid.New(),
				UserName: "User3",
				Approved: sql.NullBool{Bool: false, Valid: false},
			},
			expected: cmkapi.WorkflowApproverDecisionPENDING,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiApprover := workflow.ApproverToAPI(tt.input)
			assert.Equal(t, tt.input.UserID, apiApprover.Id)
			assert.Equal(t, tt.input.UserName, *apiApprover.Name)
			assert.Equal(t, tt.expected, apiApprover.Decision)
		})
	}
}
