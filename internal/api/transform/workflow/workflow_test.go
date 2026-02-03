package workflow_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/workflow"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

func TestWorkflow_ToAPI(t *testing.T) {
	workflowMutator := testutils.NewMutator(func() model.Workflow {
		return model.Workflow{
			ID:           uuid.New(),
			InitiatorID:  uuid.NewString(),
			State:        "INITIAL",
			ActionType:   "LINK",
			ArtifactType: "SYSTEM",
			ArtifactID:   uuid.New(),
			Parameters:   "ENABLED",
		}
	})

	expires := time.Now().AddDate(0, 0, 30)

	tests := []struct {
		name                 string
		dbWorkflow           model.Workflow
		expectedState        cmkapi.WorkflowState
		expectedActionType   cmkapi.WorkflowActionType
		expectedArtifactType cmkapi.WorkflowArtifactType
		expectedExpiryAt     *time.Time
		errorExpected        bool
	}{
		{
			name:                 "TestWorkflow_ToAPI_Valid",
			dbWorkflow:           workflowMutator(),
			expectedState:        cmkapi.WorkflowStateEnumINITIAL,
			expectedActionType:   cmkapi.WorkflowActionTypeEnumLINK,
			expectedArtifactType: cmkapi.WorkflowArtifactTypeEnumSYSTEM,
		},
		{
			name: "TestWorkflow_ToAPI_Lowercase",
			dbWorkflow: workflowMutator(func(w *model.Workflow) {
				w.State = "initial"
				w.ActionType = "link"
				w.ArtifactType = "system"
			}),
			expectedState:        cmkapi.WorkflowStateEnumINITIAL,
			expectedActionType:   cmkapi.WorkflowActionTypeEnumLINK,
			expectedArtifactType: cmkapi.WorkflowArtifactTypeEnumSYSTEM,
		},
		{
			name: "TestWorkflow_ToAPI_WithFailureReason",
			dbWorkflow: workflowMutator(func(w *model.Workflow) {
				w.State = "FAILED"
				w.FailureReason = "Failed to connect system"
			}),
			expectedState:        cmkapi.WorkflowStateEnumFAILED,
			expectedActionType:   cmkapi.WorkflowActionTypeEnumLINK,
			expectedArtifactType: cmkapi.WorkflowArtifactTypeEnumSYSTEM,
		},
		{
			name: "TestWorkflow_ToAPI_WithExpires",
			dbWorkflow: workflowMutator(func(w *model.Workflow) {
				w.State = "FAILED"
				w.ExpiryDate = &expires
			}),
			expectedState:        cmkapi.WorkflowStateEnumFAILED,
			expectedActionType:   cmkapi.WorkflowActionTypeEnumLINK,
			expectedArtifactType: cmkapi.WorkflowArtifactTypeEnumSYSTEM,
			expectedExpiryAt:     ptr.PointTo(expires),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiWorkflow, err := workflow.ToAPI(tt.dbWorkflow)

			if tt.errorExpected {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				assert.Equal(t, tt.dbWorkflow.ID, *apiWorkflow.Id)
				assert.Equal(t, tt.dbWorkflow.InitiatorID, apiWorkflow.InitiatorID)
				assert.Equal(t, tt.expectedState, apiWorkflow.State)
				assert.Equal(t, tt.expectedActionType, apiWorkflow.ActionType)
				assert.Equal(t, tt.expectedArtifactType, apiWorkflow.ArtifactType)
				assert.Equal(t, tt.dbWorkflow.ArtifactID, apiWorkflow.ArtifactID)
				assert.Equal(t, tt.dbWorkflow.Parameters, *apiWorkflow.Parameters)
				assert.Equal(t, tt.dbWorkflow.FailureReason, *apiWorkflow.FailureReason)
				if tt.expectedExpiryAt != nil {
					assert.Equal(t, *tt.expectedExpiryAt, *apiWorkflow.ExpiresAt)
				}
			}
		})
	}
}

func TestWorkflow_FromAPI(t *testing.T) {
	apiWorkflowMutator := testutils.NewMutator(func() cmkapi.WorkflowBody {
		return cmkapi.WorkflowBody{
			ActionType:   cmkapi.WorkflowActionTypeEnumLINK,
			ArtifactType: cmkapi.WorkflowArtifactTypeEnumSYSTEM,
			ArtifactID:   uuid.New(),
			Parameters:   ptr.PointTo("ENABLED"),
		}
	})

	tests := []struct {
		name          string
		apiWorkflow   cmkapi.WorkflowBody
		ctxFn         func(ctx context.Context) context.Context
		errorExpected bool
	}{
		{
			name:        "Should be valid with context",
			apiWorkflow: apiWorkflowMutator(),
			ctxFn: func(ctx context.Context) context.Context {
				return cmkcontext.InjectClientData(ctx, &auth.ClientData{Identifier: "User-ID"}, nil)
			},
		},
		{
			name:          "Should be invalid if missing context",
			apiWorkflow:   apiWorkflowMutator(),
			errorExpected: true,
		},
		{
			name: "TestWorkflow_FromAPExpiry",
			apiWorkflow: apiWorkflowMutator(func(w *cmkapi.WorkflowBody) {
				w.ExpiresAt = ptr.PointTo(time.Now())
			}),
			ctxFn: func(ctx context.Context) context.Context {
				return cmkcontext.InjectClientData(ctx, &auth.ClientData{Identifier: "User-ID"}, nil)
			},
		},
		{
			name: "TestWorkflow_FromAPI_ExceededExpiry",
			apiWorkflow: apiWorkflowMutator(func(w *cmkapi.WorkflowBody) {
				w.ExpiresAt = ptr.PointTo(time.Now().AddDate(0, 0, 31))
			}),
			ctxFn: func(ctx context.Context) context.Context {
				return cmkcontext.InjectClientData(ctx, &auth.ClientData{Identifier: "User-ID"}, nil)
			},
			errorExpected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctx context.Context
			if tt.ctxFn == nil {
				ctx = t.Context()
			} else {
				ctx = tt.ctxFn(t.Context())
			}

			w, err := workflow.FromAPI(ctx, tt.apiWorkflow, 29, 30)

			if tt.errorExpected {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, string(tt.apiWorkflow.ActionType), w.ActionType)
				assert.Equal(t, string(tt.apiWorkflow.ArtifactType), w.ArtifactType)
				assert.Equal(t, tt.apiWorkflow.ArtifactID, w.ArtifactID)
				assert.Equal(t, *tt.apiWorkflow.Parameters, w.Parameters)
			}
		})
	}
}

func TestWorkflow_FromAPI_Expires(t *testing.T) {
	apiWorkflowMutator := testutils.NewMutator(func() cmkapi.WorkflowBody {
		return cmkapi.WorkflowBody{
			ActionType:   cmkapi.WorkflowActionTypeEnumLINK,
			ArtifactType: cmkapi.WorkflowArtifactTypeEnumSYSTEM,
			ArtifactID:   uuid.New(),
			Parameters:   ptr.PointTo("ENABLED"),
		}
	})

	defaultExpiry := 29
	maxExpiry := 30

	tests := []struct {
		name         string
		apiWorkflow  cmkapi.WorkflowBody
		expiredNow   bool
		expiredAtMax bool
		error        bool
	}{
		{
			name:         "TestWorkflow_FromAPExpires_Default",
			apiWorkflow:  apiWorkflowMutator(),
			expiredNow:   false,
			expiredAtMax: true,
		},
		{
			name: "TestWorkflow_FromAPExpires_Yesterday",
			apiWorkflow: apiWorkflowMutator(func(w *cmkapi.WorkflowBody) {
				w.ExpiresAt = ptr.PointTo(time.Now().AddDate(0, 0, -1))
			}),
			expiredNow:   true,
			expiredAtMax: true,
		},
		{
			name: "TestWorkflow_FromAPExpires_1DayBeforeMax",
			apiWorkflow: apiWorkflowMutator(func(w *cmkapi.WorkflowBody) {
				w.ExpiresAt = ptr.PointTo(time.Now().AddDate(0, 0, maxExpiry-1))
			}),
			expiredNow:   false,
			expiredAtMax: true,
		},
		{
			name: "TestWorkflow_FromAPExpires_AtMax",
			apiWorkflow: apiWorkflowMutator(func(w *cmkapi.WorkflowBody) {
				w.ExpiresAt = ptr.PointTo(time.Now().AddDate(0, 0, maxExpiry))
			}),
			expiredNow:   false,
			expiredAtMax: false,
		},
		{
			name: "TestWorkflow_FromAPExpires_1DayAfterMax",
			apiWorkflow: apiWorkflowMutator(func(w *cmkapi.WorkflowBody) {
				w.ExpiresAt = ptr.PointTo(time.Now().AddDate(0, 0, maxExpiry+1))
			}),
			error: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			ctx = cmkcontext.InjectClientData(ctx, &auth.ClientData{Identifier: "User-ID"}, nil)

			w, err := workflow.FromAPI(ctx, tt.apiWorkflow, defaultExpiry, maxExpiry)

			if tt.error {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				nowTestTime := time.Now()
				assert.Equal(t, tt.expiredNow, nowTestTime.After(*w.ExpiryDate))

				maxTestTime := time.Now().AddDate(0, 0, defaultExpiry)
				assert.Equal(t, tt.expiredAtMax, maxTestTime.After(*w.ExpiryDate))
			}
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
				UserID:   uuid.NewString(),
				UserName: "User1",
				Approved: sql.NullBool{Bool: true, Valid: true},
			},
			expected: cmkapi.WorkflowApproverDecisionAPPROVED,
		},
		{
			name: "Rejected",
			input: model.WorkflowApprover{
				UserID:   uuid.NewString(),
				UserName: "User2",
				Approved: sql.NullBool{Bool: false, Valid: true},
			},
			expected: cmkapi.WorkflowApproverDecisionREJECTED,
		},
		{
			name: "Pending",
			input: model.WorkflowApprover{
				UserID:   uuid.NewString(),
				UserName: "User3",
				Approved: sql.NullBool{Bool: false, Valid: false},
			},
			expected: cmkapi.WorkflowApproverDecisionPENDING,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiApprover, err := workflow.ApproverToAPI(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.input.UserID, apiApprover.Id)
			assert.Equal(t, tt.input.UserName, *apiApprover.Name)
			assert.Equal(t, tt.expected, apiApprover.Decision)
		})
	}
}
