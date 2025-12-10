package client

import (
	"context"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
	notificationv1 "github.com/openkcm/plugin-sdk/proto/plugin/notification/v1"

	cmkcatalog "github.tools.sap/kms/cmk/internal/grpc/catalog"
	"github.tools.sap/kms/cmk/internal/log"
)

const (
	PluginName = "NOTIFICATION"
)

type Data struct {
	Recipients []string
	Subject    string
	Body       string
}

type Client struct {
	notificationClient notificationv1.NotificationServiceClient
}

func New(
	ctx context.Context,
	catalog *plugincatalog.Catalog,
) *Client {
	client, err := createNotificationClient(catalog)
	if err != nil {
		log.Error(ctx, "Creating notification client", err)
	}

	return &Client{
		notificationClient: client,
	}
}

//nolint:ireturn
func createNotificationClient(
	catalog *plugincatalog.Catalog,
) (notificationv1.NotificationServiceClient, error) {
	notification := catalog.LookupByTypeAndName(notificationv1.Type, PluginName)
	if notification == nil {
		return nil, cmkcatalog.ErrNoPluginInCatalog
	}

	return notificationv1.NewNotificationServiceClient(notification.ClientConnection()), nil
}

func (c *Client) CreateNotification(ctx context.Context, data Data) error {
	_, err := c.notificationClient.SendNotification(ctx, &notificationv1.SendNotificationRequest{
		NotificationType: notificationv1.NotificationType_NOTIFICATION_TYPE_EMAIL,
		Recipients:       data.Recipients,
		Subject:          data.Subject,
		Body:             data.Body,
	})

	return err
}
