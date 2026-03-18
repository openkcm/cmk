package tasks

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/notifier/client"
)

type NotificationClient interface {
	CreateNotification(ctx context.Context, notif client.Data) error
}

type notificationSender struct {
	client NotificationClient
}

func NewNotificationSender(
	client NotificationClient,
) async.TaskHandler {
	return &notificationSender{
		client: client,
	}
}

func (n *notificationSender) ProcessTask(ctx context.Context, task *asynq.Task) error {
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

func (n *notificationSender) TaskType() string {
	return config.TypeSendNotifications
}
