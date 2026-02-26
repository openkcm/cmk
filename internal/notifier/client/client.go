package client

import (
	"context"
	"errors"

	notificationv1 "github.com/openkcm/plugin-sdk/proto/plugin/notification/v1"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
)

var (
	ErrLoadNotificationPlugin = errors.New("failed to load notification plugin")
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
	svcRegistry *cmkpluginregistry.Registry,
) *Client {
	client, err := createNotificationClient(svcRegistry)
	if err != nil {
		log.Error(ctx, "Creating notification client", err)
	}

	return &Client{
		notificationClient: client,
	}
}

//nolint:ireturn
func createNotificationClient(
	svcRegistry *cmkpluginregistry.Registry,
) (notificationv1.NotificationServiceClient, error) {
	plugins := svcRegistry.LookupByType(notificationv1.Type)
	if len(plugins) == 0 {
		return nil, cmkpluginregistry.ErrNoPluginInCatalog
	}
	if len(plugins) > 1 {
		return nil, errs.Wrapf(ErrLoadNotificationPlugin, "multiple notification plugins found in catalog")
	}

	notification := plugins[0]

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
