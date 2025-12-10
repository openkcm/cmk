package tasks

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"

	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/log"
	"github.tools.sap/kms/cmk/internal/notifier/client"
)

type NotificationClient interface {
	CreateNotification(ctx context.Context, notif client.Data) error
}

type NotificationSender struct {
	client NotificationClient
}

func NewNotificationSender(
	client NotificationClient,
) *NotificationSender {
	return &NotificationSender{
		client: client,
	}
}

func (n *NotificationSender) ProcessTask(ctx context.Context, task *asynq.Task) error {
	log.Info(ctx, "starting notification sender task")

	var data client.Data

	err := json.Unmarshal(task.Payload(), &data)
	if err != nil {
		log.Error(ctx, "failed to unmarshal notification payload", err)
		return err
	}

	err = n.client.CreateNotification(ctx, data)
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
