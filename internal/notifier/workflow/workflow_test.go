package workflow_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/notifier/client"
	"github.tools.sap/kms/cmk/internal/notifier/workflow"
	wf "github.tools.sap/kms/cmk/internal/workflow"
)

const (
	testMessage    = "Test message"
	testActionText = "Test action text"
	testSubject    = "Test Identifier"
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
	creator, err := workflow.NewWorkflowCreator()
	assert.NoError(t, err)

	assert.NotNil(t, creator)
	assert.NotNil(t, creator.Template())
}

func TestCreator_CreateTask(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator()
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
		name        string
		transition  wf.Transition
		recipients  []string
		expectError bool
	}{
		{
			name:        "create transition",
			transition:  wf.TransitionCreate,
			recipients:  []string{"approver@example.com"},
			expectError: false,
		},
		{
			name:        "approve transition",
			transition:  wf.TransitionApprove,
			recipients:  []string{"initiator@example.com"},
			expectError: false,
		},
		{
			name:        "reject transition",
			transition:  wf.TransitionReject,
			recipients:  []string{"initiator@example.com"},
			expectError: false,
		},
		{
			name:        "confirm transition",
			transition:  wf.TransitionConfirm,
			recipients:  []string{"approver@example.com"},
			expectError: false,
		},
		{
			name:        "revoke transition",
			transition:  wf.TransitionRevoke,
			recipients:  []string{"approver@example.com"},
			expectError: false,
		},
		{
			name:        "unsupported transition",
			transition:  wf.Transition("unsupported"),
			recipients:  []string{"test@example.com"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := workflow.NotificationData{
				Tenant:     testTenant,
				Workflow:   testWorkflow,
				Transition: tt.transition,
			}

			task, err := creator.CreateTask(data, tt.recipients)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, task)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, task)
				assert.Equal(t, config.TypeSendNotifications, task.Type())

				// Verify task payload
				var notifData client.Data

				err = json.Unmarshal(task.Payload(), &notifData)
				assert.NoError(t, err)
				assert.Equal(t, tt.recipients, notifData.Recipients)
				assert.NotEmpty(t, notifData.Subject)
				assert.NotEmpty(t, notifData.Body)
			}
		})
	}
}

func TestCreator_createWorkflowCreatedTask(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator()
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

	recipients := []string{"approver@example.com"}

	task, err := creator.CreateWorkflowCreatedTask(data, recipients)

	assert.NoError(t, err)
	assert.NotNil(t, task)

	var notifData client.Data

	err = json.Unmarshal(task.Payload(), &notifData)
	assert.NoError(t, err)

	expectedSubject := fmt.Sprintf(
		"Workflow Approval Required - %s %s",
		data.Workflow.ActionType,
		data.Workflow.ArtifactType,
	)

	assert.Equal(t, recipients, notifData.Recipients)
	assert.Equal(t, expectedSubject, notifData.Subject)
	assert.Contains(t, notifData.Body, "A new workflow requires your approval")
	assert.Contains(t, notifData.Body, "Action Required")
}

func TestCreator_createWorkflowApprovedTask(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator()
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
		Transition: wf.TransitionApprove,
	}

	recipients := []string{"initiator@example.com"}

	task, err := creator.CreateWorkflowApprovedTask(data, recipients)

	assert.NoError(t, err)
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
	assert.Contains(t, notifData.Body, "approved and is now being processed")
	assert.Contains(t, notifData.Body, "track the progress")
}

func TestCreator_createWorkflowRejectedTask(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator()
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
	creator, err := workflow.NewWorkflowCreator()
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
	creator, err := workflow.NewWorkflowCreator()
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
	creator, err := workflow.NewWorkflowCreator()
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

	body, err := creator.CreateHTMLBody(data, testMessage, testActionText)

	assert.NoError(t, err)
	assert.NotEmpty(t, body)

	// Verify template data is included in the body
	assert.Contains(t, body, "CMK Workflow Notification")
	assert.Contains(t, body, testMessage)
	assert.Contains(t, body, testActionText)
	assert.Contains(t, body, workflowID.String())
	assert.Contains(t, body, data.Tenant.ID)
	assert.Contains(t, body, data.Tenant.Region)
	assert.Contains(t, body, data.Workflow.ActionType)
	assert.Contains(t, body, artifactID.String())
	assert.Contains(t, body, data.Workflow.ArtifactType)
}

func TestCreator_createNotificationTask(t *testing.T) {
	creator, err := workflow.NewWorkflowCreator()
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
	creator, err := workflow.NewWorkflowCreator()
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
