package tasks

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	"github.com/openkcm/plugin-sdk/api/service/notification"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/log"
)

type NotificationSender struct {
	service notification.Notification
}

func NewNotificationSender(
	service notification.Notification,
) *NotificationSender {
	return &NotificationSender{
		service: service,
	}
}

type emailNotificationData struct {
	Recipients []string
	Subject    string
	Body       string
}

func (n *NotificationSender) ProcessTask(ctx context.Context, task *asynq.Task) error {
	log.Info(ctx, "starting notification sender task")

	var data emailNotificationData

	err := json.Unmarshal(task.Payload(), &data)
	if err != nil {
		log.Error(ctx, "failed to unmarshal notification payload", err)
		return err
	}

	_, err = n.service.Send(ctx, &notification.SendNotificationRequest{
		Type:       notification.Email,
		Recipients: data.Recipients,
		Subject:    data.Subject,
		Body:       data.Body,
	})
	if err != nil {
		log.Error(ctx, "failed to create notification", err)
		return err
	}

	log.Info(ctx, "notification sent successfully")

	return nil
}

func (n *NotificationSender) TaskType() string {
	return config.TypeSendNotifications
}
