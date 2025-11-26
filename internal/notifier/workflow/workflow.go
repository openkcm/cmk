package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"

	"github.com/hibiken/asynq"

	_ "embed"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	notifClient "github.com/openkcm/cmk/internal/notifier/client"
	wf "github.com/openkcm/cmk/internal/workflow"
)

//go:embed templates/workflow_notification.html
var workflowNotificationTemplate string

type Creator struct {
	template *template.Template
}

var (
	ErrUnsupportedWorkflowTransition = errors.New("unsupported transition")
	ErrParsingTemplate               = errors.New("error parsing notification template")
	ErrExecutingTemplate             = errors.New("error executing notification template")
	ErrMarshallingPayload            = errors.New("error marshalling notification payload")
)

// NotificationData holds workflow-specific notification data
type NotificationData struct {
	Tenant     model.Tenant
	Workflow   model.Workflow
	Transition wf.Transition
}

func (w NotificationData) GetType() string {
	return string(w.Transition)
}

// NotificationTemplateData contains all data needed for the HTML template
type NotificationTemplateData struct {
	HeaderTitle  string
	Message      string
	InfoTitle    string
	WorkflowID   string
	TenantID     string
	TenantRegion string
	ActionType   string
	ArtifactID   string
	ArtifactType string
	ActionText   string
	Parameters   string
}

func NewWorkflowCreator() (*Creator, error) {
	tmpl, err := template.New("workflow_notification").Parse(workflowNotificationTemplate)
	if err != nil {
		return nil, errs.Wrap(ErrParsingTemplate, err)
	}

	return &Creator{
		template: tmpl,
	}, nil
}

func (w *Creator) CreateTask(data NotificationData, recipients []string) (*asynq.Task, error) {
	switch data.Transition {
	case wf.TransitionCreate:
		return w.createWorkflowCreatedTask(data, recipients)
	case wf.TransitionApprove:
		return w.createWorkflowApprovedTask(data, recipients)
	case wf.TransitionReject:
		return w.createWorkflowRejectedTask(data, recipients)
	case wf.TransitionConfirm:
		return w.createWorkflowConfirmedTask(data, recipients)
	case wf.TransitionRevoke:
		return w.createWorkflowRevokedTask(data, recipients)
	default:
		return nil, ErrUnsupportedWorkflowTransition
	}
}

func (w *Creator) createWorkflowCreatedTask(data NotificationData, recipients []string) (*asynq.Task, error) {
	subject := fmt.Sprintf(
		"Workflow Approval Required - %s %s",
		data.Workflow.ActionType,
		data.Workflow.ArtifactType,
	)

	message := "A new workflow requires your approval."
	actionText := "Action Required: Please review and approve or deny this workflow in the CMK portal."

	return w.createNotificationTask(data, recipients, subject, message, actionText)
}

func (w *Creator) createWorkflowApprovedTask(data NotificationData, recipients []string) (*asynq.Task, error) {
	subject := fmt.Sprintf(
		"Workflow Approved - %s %s",
		data.Workflow.ActionType,
		data.Workflow.ArtifactType,
	)

	message := "Your workflow has been approved and is now being processed."
	actionText := "You can track the progress in the CMK portal."

	return w.createNotificationTask(data, recipients, subject, message, actionText)
}

func (w *Creator) createWorkflowRejectedTask(data NotificationData, recipients []string) (*asynq.Task, error) {
	subject := fmt.Sprintf(
		"Workflow Rejected - %s %s",
		data.Workflow.ActionType,
		data.Workflow.ArtifactType,
	)

	message := "Your workflow has been rejected. Please review and resubmit if necessary."
	actionText := "Review the rejection reason and make necessary changes in the CMK portal."

	return w.createNotificationTask(data, recipients, subject, message, actionText)
}

func (w *Creator) createWorkflowConfirmedTask(data NotificationData, recipients []string) (*asynq.Task, error) {
	var subject, message, actionText string

	switch wf.State(data.Workflow.State) {
	case wf.StateSuccessful:
		subject = fmt.Sprintf(
			"Workflow Successful - %s %s",
			data.Workflow.ActionType,
			data.Workflow.ArtifactType,
		)
		message = "Your workflow has been confirmed and completed successfully."
		actionText = "No further action required. You can view the details in the CMK portal."

	case wf.StateFailed:
		subject = fmt.Sprintf(
			"Workflow Failed - %s %s",
			data.Workflow.ActionType,
			data.Workflow.ArtifactType,
		)
		message = "Your workflow has been confirmed but failed during execution." +
			" Please check the details and take necessary actions."
		actionText = "Review the failure reason and contact support if needed." +
			" You can view the details in the CMK portal."
	default:
		subject = fmt.Sprintf(
			"Workflow Confirmed - %s %s",
			data.Workflow.ActionType,
			data.Workflow.ArtifactType,
		)
		message = "Your workflow has been confirmed."
		actionText = "Please check the CMK portal for more details."
	}

	return w.createNotificationTask(data, recipients, subject, message, actionText)
}

func (w *Creator) createWorkflowRevokedTask(data NotificationData, recipients []string) (*asynq.Task, error) {
	subject := fmt.Sprintf(
		"Workflow Revoked - %s %s",
		data.Workflow.ActionType,
		data.Workflow.ArtifactType,
	)

	message := "Your workflow has been revoked and is no longer active."
	actionText := "Please contact your administrator if you have questions about this revocation."

	return w.createNotificationTask(data, recipients, subject, message, actionText)
}

func (w *Creator) createNotificationTask(
	data NotificationData,
	recipients []string,
	subject, message, actionText string) (*asynq.Task, error) {
	body, err := w.createHTMLBody(data, message, actionText)
	if err != nil {
		return nil, err
	}

	d := notifClient.Data{
		Recipients: recipients,
		Subject:    subject,
		Body:       body,
	}

	payload, err := json.Marshal(d)
	if err != nil {
		return nil, errs.Wrap(ErrMarshallingPayload, err)
	}

	return asynq.NewTask(config.TypeSendNotifications, payload), nil
}

func (w *Creator) createHTMLBody(data NotificationData, message, actionText string) (string, error) {
	templateData := NotificationTemplateData{
		HeaderTitle:  "CMK Workflow Notification",
		Message:      message,
		InfoTitle:    "Workflow Information",
		WorkflowID:   data.Workflow.ID.String(),
		TenantID:     data.Tenant.ID,
		TenantRegion: data.Tenant.Region,
		ActionType:   data.Workflow.ActionType,
		ArtifactID:   data.Workflow.ArtifactID.String(),
		ArtifactType: data.Workflow.ArtifactType,
		ActionText:   actionText,
		Parameters:   data.Workflow.Parameters,
	}

	var buf bytes.Buffer

	err := w.template.Execute(&buf, templateData)
	if err != nil {
		return "", errs.Wrap(ErrExecutingTemplate, err)
	}

	return buf.String(), nil
}
