package workflow

import (
	"context"
	"html/template"

	"github.com/hibiken/asynq"
)

// Export private functions for testing

func (w *Creator) CreateWorkflowCreatedTask(ctx context.Context, data NotificationData, recipients []string) (*asynq.Task, error) {
	return w.createWorkflowCreatedTask(ctx, data, recipients)
}

func (w *Creator) CreateWorkflowApprovedTask(ctx context.Context, data NotificationData, recipients []string) (*asynq.Task, error) {
	return w.createWorkflowApprovedTask(ctx, data, recipients)
}

func (w *Creator) CreateWorkflowRejectedTask(ctx context.Context, data NotificationData, recipients []string) (*asynq.Task, error) {
	return w.createWorkflowRejectedTask(ctx, data, recipients)
}

func (w *Creator) CreateWorkflowConfirmedTask(ctx context.Context, data NotificationData, recipients []string) (*asynq.Task, error) {
	return w.createWorkflowConfirmedTask(ctx, data, recipients)
}

func (w *Creator) CreateWorkflowRevokedTask(ctx context.Context, data NotificationData, recipients []string) (*asynq.Task, error) {
	return w.createWorkflowRevokedTask(ctx, data, recipients)
}

func (w *Creator) CreateNotificationTask(
	ctx context.Context,
	data NotificationData,
	recipients []string,
	subject, message, actionText string,
) (*asynq.Task, error) {
	return w.createNotificationTask(ctx, data, recipients, subject, message, actionText)
}

func (w *Creator) CreateHTMLBody(ctx context.Context, data NotificationData, message, actionText string) (string, error) {
	return w.createHTMLBody(ctx, data, message, actionText)
}

// Export template field for testing
func (w *Creator) Template() *template.Template {
	return w.template
}
