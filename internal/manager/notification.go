package manager

import (
	"context"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
	notificationv1 "github.com/openkcm/plugin-sdk/proto/plugin/notification/v1"

	"github.com/openkcm/cmk/internal/log"
)

const (
	NotificationPluginName = "NOTIFICATION"
)

type NotificationManager struct {
	notificationClient notificationv1.NotificationServiceClient
}

func NewNotificationManager(
	ctx context.Context,
	catalog *plugincatalog.Catalog,
) *NotificationManager {
	client, err := createNotificationClient(catalog)
	if err != nil {
		log.Error(ctx, "Creating notification client", err)
	}

	return &NotificationManager{
		notificationClient: client,
	}
}

//nolint:ireturn
func createNotificationClient(
	catalog *plugincatalog.Catalog,
) (notificationv1.NotificationServiceClient, error) {
	notifcation := catalog.LookupByTypeAndName(notificationv1.Type, NotificationPluginName)
	if notifcation == nil {
		return nil, ErrNoPluginInCatalog
	}

	return notificationv1.NewNotificationServiceClient(notifcation.ClientConnection()), nil
}

type ANSNotification struct {
	Recipients []string
	Subject    string
	Body       string
}

func (m *NotificationManager) CreateNotification(ctx context.Context, notif ANSNotification) error {
	_, err := m.notificationClient.SendNotification(ctx, &notificationv1.SendNotificationRequest{
		NotificationType: notificationv1.NotificationType_NOTIFICATION_TYPE_EMAIL,
		Recipients:       notif.Recipients,
		Subject:          notif.Subject,
		Body:             notif.Body,
	})

	return err
}
