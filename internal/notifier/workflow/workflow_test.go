package workflow_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/notifier/client"
	"github.com/openkcm/cmk/internal/notifier/workflow"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	wf "github.com/openkcm/cmk/internal/workflow"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

const (
	testMessage    = "Test message"
	testActionText = "Test action text"
	testSubject    = "Test Identifier"
	initiatorID    = "test-initiator-id"
)

var testConfig = &config.Config{
	Landscape: config.Landscape{
		Name:      "Staging",
		UIBaseUrl: "https://cmk-staging.example.com/#",
	},
	Notification: config.Notification{
		ServiceName: "Key Management Service",
		PortalName:  "CMK Portal",
		SignOff:     "Your Key Management Service Team",
	},
}

// newTestIDM returns a TestIdentityManagement
func newTestIDM() *testplugins.TestIdentityManagement {
	idm := testplugins.NewTestIdentityManagement()
	idm.PutUser(identitymanagement.User{ID: initiatorID})
	return idm
}

func TestNotificationData_GetType(t *testing.T) {
	tests := []struct {
		name       string
		transition wf.Transition
		expected   string
	}{
		{
			name:       "create transition",
			transition: wf.TransitionCreate,
			expected:   string(wf.TransitionCreate),
		},
		{
			name:       "approve transition",
			transition: wf.TransitionApprove,
			expected:   string(wf.TransitionApprove),
		},
		{
			name:       "reject transition",
			transition: wf.TransitionReject,
			expected:   string(wf.TransitionReject),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := workflow.NotificationData{
				Transition: tt.transition,
			}
			assert.Equal(t, tt.expected, data.GetType())
		})
	}
}

func TestNewWorkflowCreator(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator(testConfig, newTestIDM())
	assert.NoError(t, err)

	assert.NotNil(t, creator)
	assert.NotNil(t, creator.Template())
}

func TestCreator_CreateTask(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator(testConfig, newTestIDM())
	assert.NoError(t, err)

	workflowID := uuid.New()
	artifactID := uuid.New()

	testWorkflow := model.Workflow{
		ID:           workflowID,
		InitiatorID:  initiatorID,
		ActionType:   model.WorkflowActionTypeDelete,
		ArtifactType: model.WorkflowArtifactTypeKey,
		ArtifactID:   artifactID,
		State:        model.WorkflowStateSuccessful,
	}

	testTenant := model.Tenant{
		ID: "test-tenant",
	}

	tests := []struct {
		name          string
		transition    wf.Transition
		recipients    []string
		expectError   bool
		wfState       model.WorkflowState
		expectNilTask bool
	}{
		{
			name:          "create transition",
			transition:    wf.TransitionCreate,
			recipients:    []string{"approver@example.com"},
			expectError:   false,
			wfState:       model.WorkflowStateWaitApproval,
			expectNilTask: false,
		},
		{
			name:          "approve transition - WAIT_CONFIRMATION state (email sent)",
			transition:    wf.TransitionApprove,
			recipients:    []string{"initiator@example.com"},
			expectError:   false,
			wfState:       model.WorkflowStateWaitConfirmation,
			expectNilTask: false,
		},
		{
			name:          "approve transition - WAIT_APPROVAL state (no email)",
			transition:    wf.TransitionApprove,
			recipients:    []string{"initiator@example.com"},
			expectError:   false,
			wfState:       model.WorkflowStateWaitApproval,
			expectNilTask: true,
		},
		{
			name:          "reject transition",
			transition:    wf.TransitionReject,
			recipients:    []string{"initiator@example.com"},
			expectError:   false,
			wfState:       model.WorkflowStateRejected,
			expectNilTask: false,
		},
		{
			name:          "confirm transition",
			transition:    wf.TransitionConfirm,
			recipients:    []string{"approver@example.com"},
			expectError:   false,
			wfState:       model.WorkflowStateSuccessful,
			expectNilTask: false,
		},
		{
			name:          "revoke transition",
			transition:    wf.TransitionRevoke,
			recipients:    []string{"approver@example.com"},
			expectError:   false,
			wfState:       model.WorkflowStateRevoked,
			expectNilTask: false,
		},
		{
			name:          "unsupported transition",
			transition:    wf.Transition("unsupported"),
			recipients:    []string{"test@example.com"},
			expectError:   true,
			wfState:       model.WorkflowStateWaitApproval,
			expectNilTask: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflowData := testWorkflow
			workflowData.State = tt.wfState

			data := workflow.NotificationData{
				Tenant:     testTenant,
				Workflow:   workflowData,
				Transition: tt.transition,
			}

			ctx := cmkcontext.InjectBusinessUserData(t.Context(), &auth.ClientData{Identifier: "User-ID"}, nil)
			task, err := creator.CreateTask(ctx, data, tt.recipients)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, task)
				return
			}

			assert.NoError(t, err)

			if tt.expectNilTask {
				assert.Nil(t, task)
				return
			}

			assert.NotNil(t, task)
			assert.Equal(t, config.TypeSendNotifications, task.Type())

			// Verify task payload
			var notifData client.Data

			err = json.Unmarshal(task.Payload(), &notifData)
			assert.NoError(t, err)
			assert.Equal(t, tt.recipients, notifData.Recipients)
			assert.NotEmpty(t, notifData.Subject)
			assert.NotEmpty(t, notifData.Body)
		})
	}
}

func TestCreator_createWorkflowCreatedTask(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator(testConfig, newTestIDM())
	assert.NoError(t, err)

	artifactID := uuid.New()
	keyConfigID := uuid.NewString()

	tests := []struct {
		name              string
		actionType        model.WorkflowActionType
		artifactType      model.WorkflowArtifactType
		artifactName      *string
		parameters        string
		expectedInSubject string
	}{
		{
			name:              "DELETE action with artifact name",
			actionType:        model.WorkflowActionTypeDelete,
			artifactType:      model.WorkflowArtifactTypeKey,
			artifactName:      new("Test Key"),
			parameters:        "",
			expectedInSubject: "Delete Key: 'Test Key'",
		},
		{
			name:              "LINK SYSTEM with artifact name to key configuration",
			actionType:        model.WorkflowActionTypeLink,
			artifactType:      model.WorkflowArtifactTypeSystem,
			artifactName:      new("Production System"),
			parameters:        keyConfigID,
			expectedInSubject: "Connect System: 'Production System'",
		},
		{
			name:              "UNLINK SYSTEM",
			actionType:        model.WorkflowActionTypeUnlink,
			artifactType:      model.WorkflowArtifactTypeSystem,
			artifactName:      nil,
			parameters:        "",
			expectedInSubject: "Disconnect System",
		},
		{
			name:              "SWITCH SYSTEM with artifact name to key configuration",
			actionType:        model.WorkflowActionTypeSwitch,
			artifactType:      model.WorkflowArtifactTypeSystem,
			artifactName:      new("Staging System"),
			parameters:        keyConfigID,
			expectedInSubject: "Switch System: 'Staging System'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflowID := uuid.New()

			data := workflow.NotificationData{
				Tenant: model.Tenant{
					ID: "test-tenant",
				},
				Workflow: model.Workflow{
					ID:           workflowID,
					InitiatorID:  initiatorID,
					ActionType:   tt.actionType,
					ArtifactType: tt.artifactType,
					ArtifactName: tt.artifactName,
					ArtifactID:   artifactID,
					Parameters:   tt.parameters,
				},
				Transition: wf.TransitionCreate,
			}

			recipients := []string{"approver@example.com"}

			ctx := cmkcontext.InjectBusinessUserData(t.Context(), &auth.ClientData{Identifier: "User-ID"}, nil)
			task, err := creator.CreateWorkflowCreatedTask(ctx, data, recipients)

			assert.NoError(t, err)
			assert.NotNil(t, task)

			var notifData client.Data

			err = json.Unmarshal(task.Payload(), &notifData)
			assert.NoError(t, err)

			expectedSubject := "Workflow Approval Required - " + tt.expectedInSubject

			assert.Equal(t, recipients, notifData.Recipients)
			assert.Equal(t, expectedSubject, notifData.Subject)
			assert.Contains(t, notifData.Body, "A new workflow requires your approval")
			assert.Contains(t, notifData.Body, "Action Required")
		})
	}
}

func TestCreator_createWorkflowApprovedTask(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator(testConfig, newTestIDM())
	assert.NoError(t, err)

	workflowID := uuid.New()
	artifactID := uuid.New()

	tests := []struct {
		name                string
		workflowState       model.WorkflowState
		shouldSendEmail     bool
		expectedMessagePart string
		expectedActionPart  string
	}{
		{
			name:            "StateWaitApproval - no email sent for partial approval",
			workflowState:   model.WorkflowStateWaitApproval,
			shouldSendEmail: false,
		},
		{
			name:                "StateWaitConfirmation - email sent when threshold met",
			workflowState:       model.WorkflowStateWaitConfirmation,
			shouldSendEmail:     true,
			expectedMessagePart: "Your workflow has been fully approved.",
			expectedActionPart:  "ready for confirmation",
		},
		{
			name:            "StateExecuting - no email sent",
			workflowState:   model.WorkflowStateExecuting,
			shouldSendEmail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := workflow.NotificationData{
				Tenant: model.Tenant{
					ID: "test-tenant",
				},
				Workflow: model.Workflow{
					ID:           workflowID,
					InitiatorID:  initiatorID,
					ActionType:   model.WorkflowActionTypeDelete,
					ArtifactType: model.WorkflowArtifactTypeKey,
					ArtifactID:   artifactID,
					State:        tt.workflowState,
				},
				Transition: wf.TransitionApprove,
			}

			recipients := []string{"initiator@example.com"}

			ctx := cmkcontext.InjectBusinessUserData(t.Context(), &auth.ClientData{Identifier: "User-ID"}, nil)
			task, err := creator.CreateWorkflowApprovedTask(ctx, data, recipients)

			assert.NoError(t, err)

			if !tt.shouldSendEmail {
				// Verify no email is sent for non-WAIT_CONFIRMATION states
				assert.Nil(t, task)
				return
			}

			// For WAIT_CONFIRMATION state, verify email is sent with correct content
			assert.NotNil(t, task)

			var notifData client.Data

			err = json.Unmarshal(task.Payload(), &notifData)
			assert.NoError(t, err)

			expectedSubject := fmt.Sprintf(
				"Workflow Approved - %s %s",
				data.Workflow.ActionType.DisplayName(),
				data.Workflow.ArtifactType.DisplayName(),
			)

			assert.Equal(t, recipients, notifData.Recipients)
			assert.Equal(t, expectedSubject, notifData.Subject)
			assert.Contains(t, notifData.Body, tt.expectedMessagePart)
			assert.Contains(t, notifData.Body, tt.expectedActionPart)
		})
	}
}

func TestCreator_createWorkflowRejectedTask(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator(testConfig, newTestIDM())
	assert.NoError(t, err)

	workflowID := uuid.New()
	artifactID := uuid.New()

	data := workflow.NotificationData{
		Tenant: model.Tenant{
			ID: "test-tenant",
		},
		Workflow: model.Workflow{
			ID:           workflowID,
			InitiatorID:  initiatorID,
			ActionType:   model.WorkflowActionTypeDelete,
			ArtifactType: model.WorkflowArtifactTypeKey,
			ArtifactID:   artifactID,
		},
		Transition: wf.TransitionReject,
	}

	recipients := []string{"initiator@example.com"}

	ctx := cmkcontext.InjectBusinessUserData(t.Context(), &auth.ClientData{Identifier: "User-ID"}, nil)
	task, err := creator.CreateWorkflowRejectedTask(ctx, data, recipients)

	assert.NoError(t, err)
	assert.NotNil(t, task)

	var notifData client.Data

	err = json.Unmarshal(task.Payload(), &notifData)
	assert.NoError(t, err)

	expectedSubject := fmt.Sprintf(
		"Workflow Rejected - %s %s",
		data.Workflow.ActionType.DisplayName(),
		data.Workflow.ArtifactType.DisplayName(),
	)

	assert.Equal(t, recipients, notifData.Recipients)
	assert.Equal(t, expectedSubject, notifData.Subject)
	assert.Contains(t, notifData.Body, "has been rejected")
	assert.Contains(t, notifData.Body, "review and resubmit")
}

func TestCreator_createWorkflowConfirmedTask(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator(testConfig, newTestIDM())
	assert.NoError(t, err)

	workflowID := uuid.New()
	artifactID := uuid.New()

	tests := []struct {
		name            string
		workflowState   model.WorkflowState
		expectedSubject string
		expectedMessage string
		expectedAction  string
	}{
		{
			name:            "successful workflow",
			workflowState:   model.WorkflowStateSuccessful,
			expectedSubject: "Workflow Successful - Delete Key",
			expectedMessage: "confirmed and completed successfully",
			expectedAction:  "No further action required",
		},
		{
			name:            "failed workflow",
			workflowState:   model.WorkflowStateFailed,
			expectedSubject: "Workflow Failed - Delete Key",
			expectedMessage: "confirmed but failed during execution",
			expectedAction:  "Review the failure reason and contact support",
		},
		{
			name:            "empty state",
			workflowState:   "",
			expectedSubject: "Workflow Confirmed - Delete Key",
			expectedMessage: "The workflow has been confirmed.",
			expectedAction:  "Please check the CMK Portal for more details",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := workflow.NotificationData{
				Tenant: model.Tenant{
					ID: "test-tenant",
				},
				Workflow: model.Workflow{
					ID:           workflowID,
					InitiatorID:  initiatorID,
					ActionType:   model.WorkflowActionTypeDelete,
					ArtifactType: model.WorkflowArtifactTypeKey,
					ArtifactID:   artifactID,
					State:        tt.workflowState,
				},
				Transition: wf.TransitionConfirm,
			}

			recipients := []string{"approver@example.com"}

			ctx := cmkcontext.InjectBusinessUserData(t.Context(), &auth.ClientData{Identifier: "User-ID"}, nil)
			task, err := creator.CreateWorkflowConfirmedTask(ctx, data, recipients)

			assert.NoError(t, err)
			assert.NotNil(t, task)

			var notifData client.Data

			err = json.Unmarshal(task.Payload(), &notifData)
			assert.NoError(t, err)

			assert.Equal(t, recipients, notifData.Recipients)
			assert.Equal(t, tt.expectedSubject, notifData.Subject)
			assert.Contains(t, notifData.Body, tt.expectedMessage)
			assert.Contains(t, notifData.Body, tt.expectedAction)
		})
	}
}

func TestCreator_createWorkflowRevokedTask(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator(testConfig, newTestIDM())
	assert.NoError(t, err)

	workflowID := uuid.New()
	artifactID := uuid.New()

	data := workflow.NotificationData{
		Tenant: model.Tenant{
			ID: "test-tenant",
		},
		Workflow: model.Workflow{
			ID:           workflowID,
			InitiatorID:  initiatorID,
			ActionType:   model.WorkflowActionTypeDelete,
			ArtifactType: model.WorkflowArtifactTypeKey,
			ArtifactID:   artifactID,
		},
		Transition: wf.TransitionRevoke,
	}

	recipients := []string{"approver@example.com"}

	ctx := cmkcontext.InjectBusinessUserData(t.Context(), &auth.ClientData{Identifier: "User-ID"}, nil)
	task, err := creator.CreateWorkflowRevokedTask(ctx, data, recipients)

	assert.NoError(t, err)
	assert.NotNil(t, task)

	var notifData client.Data

	err = json.Unmarshal(task.Payload(), &notifData)
	assert.NoError(t, err)

	expectedSubject := fmt.Sprintf(
		"Workflow Revoked - %s %s",
		data.Workflow.ActionType.DisplayName(),
		data.Workflow.ArtifactType.DisplayName(),
	)

	assert.Equal(t, recipients, notifData.Recipients)
	assert.Equal(t, expectedSubject, notifData.Subject)
	assert.Contains(t, notifData.Body, "has been revoked")
	assert.Contains(t, notifData.Body, "contact your administrator")
}

func TestCreator_createHTMLBody(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator(testConfig, newTestIDM())
	assert.NoError(t, err)

	workflowID := uuid.New()
	artifactID := uuid.New()

	data := workflow.NotificationData{
		Tenant: model.Tenant{
			ID:   "test-tenant",
			Name: "test-tenant-name",
		},
		Workflow: model.Workflow{
			ID:           workflowID,
			InitiatorID:  initiatorID,
			ActionType:   model.WorkflowActionTypeDelete,
			ArtifactType: model.WorkflowArtifactTypeKey,
			ArtifactID:   artifactID,
			ArtifactName: new("Test Key"),
		},
		Transition: wf.TransitionCreate,
	}

	ctx := cmkcontext.InjectBusinessUserData(t.Context(), &auth.ClientData{Identifier: "User-ID"}, nil)
	body, err := creator.CreateHTMLBody(ctx, data, testMessage, testActionText)

	assert.NoError(t, err)
	assert.NotEmpty(t, body)

	// Verify template data is included in the body
	assert.Contains(t, body, "Customer Managed Keys - Workflow Notification")
	assert.Contains(t, body, testMessage)
	assert.Contains(t, body, testActionText)
	assert.Contains(t, body, data.Tenant.ID)
	assert.Contains(t, body, testConfig.Landscape.Region)
	assert.Contains(t, body, data.Tenant.Name)
	expectedWorkflowURL := fmt.Sprintf(
		"%s/%s/tasks/%s", testConfig.Landscape.UIBaseUrl, data.Tenant.ID, data.Workflow.ID)
	assert.Contains(t, body, expectedWorkflowURL)

	// Security text is absent when not configured
	assert.NotContains(t, body, "security-warning-marker")
}

func TestCreator_createHTMLBody_SecurityText(t *testing.T) {
	cfgWithSecurity := *testConfig
	cfgWithSecurity.Notification.SecurityText = `<p class="security-warning-marker">Do not approve unknown requests.</p>`

	creator, err := workflow.NewWorkflowCreator(&cfgWithSecurity, newTestIDM())
	assert.NoError(t, err)

	data := workflow.NotificationData{
		Tenant: model.Tenant{ID: "test-tenant"},
		Workflow: model.Workflow{
			ID:           uuid.New(),
			InitiatorID:  initiatorID,
			ActionType:   model.WorkflowActionTypeDelete,
			ArtifactType: model.WorkflowArtifactTypeKey,
			ArtifactID:   uuid.New(),
		},
		Transition: wf.TransitionCreate,
	}

	ctx := cmkcontext.InjectBusinessUserData(t.Context(), &auth.ClientData{Identifier: "User-ID"}, nil)
	body, err := creator.CreateHTMLBody(ctx, data, testMessage, testActionText)

	assert.NoError(t, err)
	assert.Contains(t, body, "security-warning-marker")
	assert.Contains(t, body, "Do not approve unknown requests.")
}

func TestCreator_createNotificationTask(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator(testConfig, newTestIDM())
	assert.NoError(t, err)

	workflowID := uuid.New()
	artifactID := uuid.New()

	data := workflow.NotificationData{
		Tenant: model.Tenant{
			ID: "test-tenant",
		},
		Workflow: model.Workflow{
			ID:           workflowID,
			InitiatorID:  initiatorID,
			ActionType:   model.WorkflowActionTypeDelete,
			ArtifactType: model.WorkflowArtifactTypeKey,
			ArtifactID:   artifactID,
		},
		Transition: wf.TransitionCreate,
	}

	recipients := []string{"test@example.com", "test2@example.com"}

	ctx := cmkcontext.InjectBusinessUserData(t.Context(), &auth.ClientData{Identifier: "User-ID"}, nil)
	task, err := creator.CreateNotificationTask(ctx, data, recipients, testSubject, testMessage, testActionText)

	assert.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, config.TypeSendNotifications, task.Type())

	var notifData client.Data

	err = json.Unmarshal(task.Payload(), &notifData)
	assert.NoError(t, err)

	assert.Equal(t, recipients, notifData.Recipients)
	assert.Equal(t, testSubject, notifData.Subject)
	assert.NotEmpty(t, notifData.Body)
	assert.Contains(t, notifData.Body, testMessage)
	assert.Contains(t, notifData.Body, testActionText)
}

func TestCreator_createNotificationTask_EmptyRecipients(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator(testConfig, newTestIDM())
	assert.NoError(t, err)

	workflowID := uuid.New()
	artifactID := uuid.New()

	data := workflow.NotificationData{
		Tenant: model.Tenant{
			ID: "test-tenant",
		},
		Workflow: model.Workflow{
			ID:           workflowID,
			InitiatorID:  initiatorID,
			ActionType:   model.WorkflowActionTypeDelete,
			ArtifactType: model.WorkflowArtifactTypeKey,
			ArtifactID:   artifactID,
		},
		Transition: wf.TransitionCreate,
	}

	var recipients []string

	ctx := cmkcontext.InjectBusinessUserData(t.Context(), &auth.ClientData{Identifier: "User-ID"}, nil)
	task, err := creator.CreateNotificationTask(ctx, data, recipients, testSubject, testMessage, testActionText)

	assert.NoError(t, err)
	assert.NotNil(t, task)

	var notifData client.Data

	err = json.Unmarshal(task.Payload(), &notifData)
	assert.NoError(t, err)

	assert.Equal(t, recipients, notifData.Recipients)
	assert.Empty(t, notifData.Recipients)
}
