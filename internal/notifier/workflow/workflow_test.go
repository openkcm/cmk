package workflow_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/notifier/client"
	"github.com/openkcm/cmk/internal/notifier/workflow"
	wf "github.com/openkcm/cmk/internal/workflow"
	"github.com/openkcm/cmk/utils/ptr"
)

const (
	testMessage    = "Test message"
	testActionText = "Test action text"
	testSubject    = "Test Identifier"
)

var (
	testConfig = &config.Config{
		Landscape: config.Landscape{
			Name:      "Staging",
			UIBaseUrl: "https://cmk-staging.example.com/#",
		},
	}
)

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
	creator, err := workflow.NewWorkflowCreator(testConfig)
	assert.NoError(t, err)

	assert.NotNil(t, creator)
	assert.NotNil(t, creator.Template())
}

func TestCreator_CreateTask(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator(testConfig)
	assert.NoError(t, err)

	workflowID := uuid.New()
	artifactID := uuid.New()

	testWorkflow := model.Workflow{
		ID:           workflowID,
		ActionType:   "DELETE",
		ArtifactType: "KEY",
		ArtifactID:   artifactID,
		State:        string(wf.StateSuccessful),
	}

	testTenant := model.Tenant{
		ID:     "test-tenant",
		Region: "us-east-1",
	}

	tests := []struct {
		name          string
		transition    wf.Transition
		recipients    []string
		expectError   bool
		wfState       string
		expectNilTask bool
	}{
		{
			name:          "create transition",
			transition:    wf.TransitionCreate,
			recipients:    []string{"approver@example.com"},
			expectError:   false,
			wfState:       string(wf.StateWaitApproval),
			expectNilTask: false,
		},
		{
			name:          "approve transition - WAIT_CONFIRMATION state (email sent)",
			transition:    wf.TransitionApprove,
			recipients:    []string{"initiator@example.com"},
			expectError:   false,
			wfState:       string(wf.StateWaitConfirmation),
			expectNilTask: false,
		},
		{
			name:          "approve transition - WAIT_APPROVAL state (no email)",
			transition:    wf.TransitionApprove,
			recipients:    []string{"initiator@example.com"},
			expectError:   false,
			wfState:       string(wf.StateWaitApproval),
			expectNilTask: true,
		},
		{
			name:          "reject transition",
			transition:    wf.TransitionReject,
			recipients:    []string{"initiator@example.com"},
			expectError:   false,
			wfState:       string(wf.StateRejected),
			expectNilTask: false,
		},
		{
			name:          "confirm transition",
			transition:    wf.TransitionConfirm,
			recipients:    []string{"approver@example.com"},
			expectError:   false,
			wfState:       string(wf.StateSuccessful),
			expectNilTask: false,
		},
		{
			name:          "revoke transition",
			transition:    wf.TransitionRevoke,
			recipients:    []string{"approver@example.com"},
			expectError:   false,
			wfState:       string(wf.StateRevoked),
			expectNilTask: false,
		},
		{
			name:          "unsupported transition",
			transition:    wf.Transition("unsupported"),
			recipients:    []string{"test@example.com"},
			expectError:   true,
			wfState:       string(wf.StateWaitApproval),
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

			task, err := creator.CreateTask(data, tt.recipients)

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
	creator, err := workflow.NewWorkflowCreator(testConfig)
	assert.NoError(t, err)

	artifactID := uuid.New()
	keyConfigID := uuid.NewString()

	tests := []struct {
		name              string
		actionType        string
		artifactType      string
		artifactName      *string
		parameters        string
		expectedInSubject string
	}{
		{
			name:              "DELETE action with artifact name",
			actionType:        "DELETE",
			artifactType:      "KEY",
			artifactName:      ptr.PointTo("Test Key"),
			parameters:        "",
			expectedInSubject: "DELETE KEY: 'Test Key'",
		},
		{
			name:              "LINK SYSTEM with artifact name to key configuration",
			actionType:        "LINK",
			artifactType:      "SYSTEM",
			artifactName:      ptr.PointTo("Production System"),
			parameters:        keyConfigID,
			expectedInSubject: "LINK SYSTEM: 'Production System'",
		},
		{
			name:              "UNLINK SYSTEM",
			actionType:        "UNLINK",
			artifactType:      "SYSTEM",
			artifactName:      nil,
			parameters:        "",
			expectedInSubject: "UNLINK SYSTEM",
		},
		{
			name:              "SWITCH SYSTEM with artifact name to key configuration",
			actionType:        "SWITCH",
			artifactType:      "SYSTEM",
			artifactName:      ptr.PointTo("Staging System"),
			parameters:        keyConfigID,
			expectedInSubject: "SWITCH SYSTEM: 'Staging System'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflowID := uuid.New()

			data := workflow.NotificationData{
				Tenant: model.Tenant{
					ID:     "test-tenant",
					Region: "us-east-1",
				},
				Workflow: model.Workflow{
					ID:            workflowID,
					InitiatorName: "initiator@example.com",
					ActionType:    tt.actionType,
					ArtifactType:  tt.artifactType,
					ArtifactName:  tt.artifactName,
					ArtifactID:    artifactID,
					Parameters:    tt.parameters,
				},
				Transition: wf.TransitionCreate,
			}

			recipients := []string{"approver@example.com"}

			task, err := creator.CreateWorkflowCreatedTask(data, recipients)

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
	creator, err := workflow.NewWorkflowCreator(testConfig)
	assert.NoError(t, err)

	workflowID := uuid.New()
	artifactID := uuid.New()

	tests := []struct {
		name                string
		workflowState       string
		shouldSendEmail     bool
		expectedMessagePart string
		expectedActionPart  string
	}{
		{
			name:            "StateWaitApproval - no email sent for partial approval",
			workflowState:   string(wf.StateWaitApproval),
			shouldSendEmail: false,
		},
		{
			name:                "StateWaitConfirmation - email sent when threshold met",
			workflowState:       string(wf.StateWaitConfirmation),
			shouldSendEmail:     true,
			expectedMessagePart: "Your workflow has been fully approved.",
			expectedActionPart:  "ready for confirmation",
		},
		{
			name:            "StateExecuting - no email sent",
			workflowState:   string(wf.StateExecuting),
			shouldSendEmail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := workflow.NotificationData{
				Tenant: model.Tenant{
					ID:     "test-tenant",
					Region: "us-east-1",
				},
				Workflow: model.Workflow{
					ID:           workflowID,
					ActionType:   "DELETE",
					ArtifactType: "KEY",
					ArtifactID:   artifactID,
					State:        tt.workflowState,
				},
				Transition: wf.TransitionApprove,
			}

			recipients := []string{"initiator@example.com"}

			task, err := creator.CreateWorkflowApprovedTask(data, recipients)

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
				data.Workflow.ActionType,
				data.Workflow.ArtifactType,
			)

			assert.Equal(t, recipients, notifData.Recipients)
			assert.Equal(t, expectedSubject, notifData.Subject)
			assert.Contains(t, notifData.Body, tt.expectedMessagePart)
			assert.Contains(t, notifData.Body, tt.expectedActionPart)
		})
	}
}

func TestCreator_createWorkflowRejectedTask(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator(testConfig)
	assert.NoError(t, err)

	workflowID := uuid.New()
	artifactID := uuid.New()

	data := workflow.NotificationData{
		Tenant: model.Tenant{
			ID:     "test-tenant",
			Region: "us-east-1",
		},
		Workflow: model.Workflow{
			ID:           workflowID,
			ActionType:   "DELETE",
			ArtifactType: "KEY",
			ArtifactID:   artifactID,
		},
		Transition: wf.TransitionReject,
	}

	recipients := []string{"initiator@example.com"}

	task, err := creator.CreateWorkflowRejectedTask(data, recipients)

	assert.NoError(t, err)
	assert.NotNil(t, task)

	var notifData client.Data

	err = json.Unmarshal(task.Payload(), &notifData)
	assert.NoError(t, err)

	expectedSubject := fmt.Sprintf(
		"Workflow Rejected - %s %s",
		data.Workflow.ActionType,
		data.Workflow.ArtifactType,
	)

	assert.Equal(t, recipients, notifData.Recipients)
	assert.Equal(t, expectedSubject, notifData.Subject)
	assert.Contains(t, notifData.Body, "has been rejected")
	assert.Contains(t, notifData.Body, "review and resubmit")
}

func TestCreator_createWorkflowConfirmedTask(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator(testConfig)
	assert.NoError(t, err)

	workflowID := uuid.New()
	artifactID := uuid.New()

	tests := []struct {
		name            string
		workflowState   string
		expectedSubject string
		expectedMessage string
		expectedAction  string
	}{
		{
			name:            "successful workflow",
			workflowState:   string(wf.StateSuccessful),
			expectedSubject: "Workflow Successful - DELETE KEY",
			expectedMessage: "confirmed and completed successfully",
			expectedAction:  "No further action required",
		},
		{
			name:            "failed workflow",
			workflowState:   string(wf.StateFailed),
			expectedSubject: "Workflow Failed - DELETE KEY",
			expectedMessage: "confirmed but failed during execution",
			expectedAction:  "Review the failure reason and contact support",
		},
		{
			name:            "empty state",
			workflowState:   "",
			expectedSubject: "Workflow Confirmed - DELETE KEY",
			expectedMessage: "Your workflow has been confirmed.",
			expectedAction:  "Please check the CMK portal for more details",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := workflow.NotificationData{
				Tenant: model.Tenant{
					ID:     "test-tenant",
					Region: "us-east-1",
				},
				Workflow: model.Workflow{
					ID:           workflowID,
					ActionType:   "DELETE",
					ArtifactType: "KEY",
					ArtifactID:   artifactID,
					State:        tt.workflowState,
				},
				Transition: wf.TransitionConfirm,
			}

			recipients := []string{"approver@example.com"}

			task, err := creator.CreateWorkflowConfirmedTask(data, recipients)

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
	creator, err := workflow.NewWorkflowCreator(testConfig)
	assert.NoError(t, err)

	workflowID := uuid.New()
	artifactID := uuid.New()

	data := workflow.NotificationData{
		Tenant: model.Tenant{
			ID:     "test-tenant",
			Region: "us-east-1",
		},
		Workflow: model.Workflow{
			ID:           workflowID,
			ActionType:   "DELETE",
			ArtifactType: "KEY",
			ArtifactID:   artifactID,
		},
		Transition: wf.TransitionRevoke,
	}

	recipients := []string{"approver@example.com"}

	task, err := creator.CreateWorkflowRevokedTask(data, recipients)

	assert.NoError(t, err)
	assert.NotNil(t, task)

	var notifData client.Data

	err = json.Unmarshal(task.Payload(), &notifData)
	assert.NoError(t, err)

	expectedSubject := fmt.Sprintf(
		"Workflow Revoked - %s %s",
		data.Workflow.ActionType,
		data.Workflow.ArtifactType,
	)

	assert.Equal(t, recipients, notifData.Recipients)
	assert.Equal(t, expectedSubject, notifData.Subject)
	assert.Contains(t, notifData.Body, "has been revoked")
	assert.Contains(t, notifData.Body, "contact your administrator")
}

func TestCreator_createHTMLBody(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator(testConfig)
	assert.NoError(t, err)

	workflowID := uuid.New()
	artifactID := uuid.New()

	data := workflow.NotificationData{
		Tenant: model.Tenant{
			ID:     "test-tenant",
			Region: "us-east-1",
		},
		Workflow: model.Workflow{
			ID:            workflowID,
			ActionType:    "DELETE",
			ArtifactType:  "KEY",
			ArtifactID:    artifactID,
			ArtifactName:  ptr.PointTo("Test Key"),
			InitiatorName: "initiator@example.com",
		},
		Transition: wf.TransitionCreate,
	}

	body, err := creator.CreateHTMLBody(data, testMessage, testActionText)

	assert.NoError(t, err)
	assert.NotEmpty(t, body)

	// Verify template data is included in the body
	assert.Contains(t, body, "CMK Workflow Notification")
	assert.Contains(t, body, testMessage)
	assert.Contains(t, body, testActionText)
	assert.Contains(t, body, data.Tenant.ID)
	assert.Contains(t, body, data.Tenant.Region)
	expectedWorkflowURL := fmt.Sprintf(
		"%s/%s/tasks/%s", testConfig.Landscape.UIBaseUrl, data.Tenant.ID, data.Workflow.ID)
	assert.Contains(t, body, expectedWorkflowURL)
}

func TestCreator_createNotificationTask(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator(testConfig)
	assert.NoError(t, err)

	workflowID := uuid.New()
	artifactID := uuid.New()

	data := workflow.NotificationData{
		Tenant: model.Tenant{
			ID:     "test-tenant",
			Region: "us-east-1",
		},
		Workflow: model.Workflow{
			ID:           workflowID,
			ActionType:   "DELETE",
			ArtifactType: "KEY",
			ArtifactID:   artifactID,
		},
		Transition: wf.TransitionCreate,
	}

	recipients := []string{"test@example.com", "test2@example.com"}

	task, err := creator.CreateNotificationTask(data, recipients, testSubject, testMessage, testActionText)

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
	creator, err := workflow.NewWorkflowCreator(testConfig)
	assert.NoError(t, err)

	workflowID := uuid.New()
	artifactID := uuid.New()

	data := workflow.NotificationData{
		Tenant: model.Tenant{
			ID:     "test-tenant",
			Region: "us-east-1",
		},
		Workflow: model.Workflow{
			ID:           workflowID,
			ActionType:   "DELETE",
			ArtifactType: "KEY",
			ArtifactID:   artifactID,
		},
		Transition: wf.TransitionCreate,
	}

	var recipients []string

	task, err := creator.CreateNotificationTask(data, recipients, testSubject, testMessage, testActionText)

	assert.NoError(t, err)
	assert.NotNil(t, task)

	var notifData client.Data

	err = json.Unmarshal(task.Payload(), &notifData)
	assert.NoError(t, err)

	assert.Equal(t, recipients, notifData.Recipients)
	assert.Empty(t, notifData.Recipients)
}
