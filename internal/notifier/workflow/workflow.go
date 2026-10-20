package workflow

import (
	"bytes"
	"context"
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
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	wf "github.com/openkcm/cmk/internal/workflow"
)

//go:embed templates/workflow_notification.html
var workflowNotificationTemplate string

type Creator struct {
	cfg      *config.Config
	template *template.Template
	idm      identitymanagement.IdentityManagement
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
	HeaderTitle         string
	Message             string
	InfoTitle           string
	WorkflowURL         string
	TenantID            string
	TenantRegion        string
	TenantName          string
	Landscape           string
	ActionText          string
	InitiatorName       string
	WorkflowDescription string
	ServiceName         string
	SignOff             string
	SecurityText        template.HTML
}

func NewWorkflowCreator(config *config.Config, idm identitymanagement.IdentityManagement) (*Creator, error) {
	tmpl, err := template.New("workflow_notification").Parse(workflowNotificationTemplate)
	if err != nil {
		return nil, errs.Wrap(ErrParsingTemplate, err)
	}

	return &Creator{
		template: tmpl,
		cfg:      config,
		idm:      idm,
	}, nil
}

func (w *Creator) CreateTask(ctx context.Context, data NotificationData, recipients []string) (*asynq.Task, error) {
	switch data.Transition {
	case wf.TransitionCreate:
		return w.createWorkflowCreatedTask(ctx, data, recipients)
	case wf.TransitionApprove:
		return w.createWorkflowApprovedTask(ctx, data, recipients)
	case wf.TransitionReject:
		return w.createWorkflowRejectedTask(ctx, data, recipients)
	case wf.TransitionConfirm:
		return w.createWorkflowConfirmedTask(ctx, data, recipients)
	case wf.TransitionRevoke:
		return w.createWorkflowRevokedTask(ctx, data, recipients)
	default:
		return nil, ErrUnsupportedWorkflowTransition
	}
}

func (w *Creator) createWorkflowCreatedTask(
	ctx context.Context,
	data NotificationData,
	recipients []string,
) (*asynq.Task, error) {
	subject := w.buildSubject("Workflow Approval Required", data.Workflow)
	message := "A new workflow requires your approval."
	actionText := fmt.Sprintf("Action Required: Please review and approve or deny this workflow in the %s.",
		w.cfg.Notification.PortalName)

	return w.createNotificationTask(ctx, data, recipients, subject, message, actionText)
}

//nolint:nilnil
func (w *Creator) createWorkflowApprovedTask(
	ctx context.Context,
	data NotificationData,
	recipients []string,
) (*asynq.Task, error) {
	// Only send email notification when minimum approvals threshold is met (WAIT_CONFIRMATION state)
	// Do not send for partial approvals (WAIT_APPROVAL state)
	if data.Workflow.State != model.WorkflowStateWaitConfirmation {
		return nil, nil
	}

	subject := w.buildSubject("Workflow Approved", data.Workflow)
	message := "Your workflow has been fully approved."
	actionText := fmt.Sprintf("The workflow is ready for confirmation. You can track the progress in the %s.",
		w.cfg.Notification.PortalName)

	return w.createNotificationTask(ctx, data, recipients, subject, message, actionText)
}

func (w *Creator) createWorkflowRejectedTask(
	ctx context.Context,
	data NotificationData,
	recipients []string,
) (*asynq.Task, error) {
	subject := w.buildSubject("Workflow Rejected", data.Workflow)
	message := "Your workflow has been rejected. Please review and resubmit if necessary."
	actionText := fmt.Sprintf("Review the rejection reason and make necessary changes in the %s.",
		w.cfg.Notification.PortalName)

	return w.createNotificationTask(ctx, data, recipients, subject, message, actionText)
}

func (w *Creator) createWorkflowConfirmedTask(
	ctx context.Context,
	data NotificationData,
	recipients []string,
) (*asynq.Task, error) {
	var subject, message, actionText string

	switch data.Workflow.State {
	case model.WorkflowStateSuccessful:
		subject = w.buildSubject("Workflow Successful", data.Workflow)
		message = "The workflow has been confirmed and completed successfully."
		actionText = fmt.Sprintf("No further action required. You can view the details in the %s.",
			w.cfg.Notification.PortalName)

	case model.WorkflowStateFailed:
		subject = w.buildSubject("Workflow Failed", data.Workflow)
		message = "The workflow has been confirmed but failed during execution." +
			" Please check the details and take necessary actions."
		actionText = fmt.Sprintf("Review the failure reason and contact support if needed."+
			" You can view the details in the %s.", w.cfg.Notification.PortalName)
	default:
		subject = w.buildSubject("Workflow Confirmed", data.Workflow)
		message = "The workflow has been confirmed."
		actionText = fmt.Sprintf("Please check the %s for more details.", w.cfg.Notification.PortalName)
	}

	return w.createNotificationTask(ctx, data, recipients, subject, message, actionText)
}

func (w *Creator) createWorkflowRevokedTask(
	ctx context.Context,
	data NotificationData,
	recipients []string,
) (*asynq.Task, error) {
	subject := w.buildSubject("Workflow Revoked", data.Workflow)
	message := "The workflow has been revoked and is no longer active."
	actionText := "Please contact your administrator if you have questions about this revocation."

	return w.createNotificationTask(ctx, data, recipients, subject, message, actionText)
}

func (w *Creator) createNotificationTask(
	ctx context.Context,
	data NotificationData,
	recipients []string,
	subject, message, actionText string,
) (*asynq.Task, error) {
	body, err := w.createHTMLBody(ctx, data, message, actionText)
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

func (w *Creator) createHTMLBody(
	ctx context.Context,
	data NotificationData,
	message, actionText string,
) (string, error) {
	workflowURL := ""
	baseURL := w.cfg.Landscape.UIBaseUrl
	if baseURL != "" {
		workflowURL = fmt.Sprintf("%s/%s/tasks/%s", baseURL, data.Tenant.ID, data.Workflow.ID)
	}

	workflowDescription, err := data.Workflow.Description(ctx, w.idm)
	if err != nil {
		return "", err
	}
	initiatorName, err := data.Workflow.GetInitiatorName(ctx, w.idm)
	if err != nil {
		return "", err
	}

	templateData := NotificationTemplateData{
		HeaderTitle:         "Customer Managed Keys - Workflow Notification",
		Message:             message,
		InfoTitle:           "Workflow Description",
		WorkflowURL:         workflowURL,
		TenantID:            data.Tenant.ID,
		TenantRegion:        w.cfg.Landscape.Region,
		TenantName:          tenantDisplayName(data.Tenant),
		Landscape:           w.cfg.Landscape.Name,
		ActionText:          actionText,
		InitiatorName:       initiatorName,
		WorkflowDescription: workflowDescription,
		ServiceName:         w.cfg.Notification.ServiceName,
		SignOff:             w.cfg.Notification.SignOff,
		SecurityText:        template.HTML(w.cfg.Notification.SecurityText), //nolint:gosec
	}

	var buf bytes.Buffer

	err = w.template.Execute(&buf, templateData)
	if err != nil {
		return "", errs.Wrap(ErrExecutingTemplate, err)
	}

	return buf.String(), nil
}

func tenantDisplayName(t model.Tenant) string {
	if t.Name != "" {
		return t.Name
	}
	return t.ID
}

// buildSubject returns "prefix - ActionType ArtifactType[: 'artifactName']"
func (w *Creator) buildSubject(prefix string, wfl model.Workflow) string {
	subject := fmt.Sprintf(
		"%s - %s %s",
		prefix,
		wfl.ActionType.DisplayName(),
		wfl.ArtifactType.DisplayName(),
	)
	if name := wfl.GetArtifactName(); name != "" {
		subject += fmt.Sprintf(": '%s'", name)
	}
	return subject
}
