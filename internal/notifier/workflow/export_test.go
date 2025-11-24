package workflow

import (
	"html/template"

	"github.com/hibiken/asynq"
)

// Export private functions for testing

func (w *Creator) CreateWorkflowCreatedTask(data NotificationData, recipients []string) (*asynq.Task, error) {
	return w.createWorkflowCreatedTask(data, recipients)
}

func (w *Creator) CreateWorkflowApprovedTask(data NotificationData, recipients []string) (*asynq.Task, error) {
	return w.createWorkflowApprovedTask(data, recipients)
}

func (w *Creator) CreateWorkflowRejectedTask(data NotificationData, recipients []string) (*asynq.Task, error) {
	return w.createWorkflowRejectedTask(data, recipients)
}

func (w *Creator) CreateWorkflowConfirmedTask(data NotificationData, recipients []string) (*asynq.Task, error) {
	return w.createWorkflowConfirmedTask(data, recipients)
}

func (w *Creator) CreateWorkflowRevokedTask(data NotificationData, recipients []string) (*asynq.Task, error) {
	return w.createWorkflowRevokedTask(data, recipients)
}

func (w *Creator) CreateNotificationTask(
	data NotificationData,
	recipients []string,
	subject, message, actionText string) (*asynq.Task, error) {
	return w.createNotificationTask(data, recipients, subject, message, actionText)
}

func (w *Creator) CreateHTMLBody(data NotificationData, message, actionText string) (string, error) {
	return w.createHTMLBody(data, message, actionText)
}

// Export template field for testing
func (w *Creator) Template() *template.Template {
	return w.template
}
