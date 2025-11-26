package client

import (
	notificationv1 "github.com/openkcm/plugin-sdk/proto/plugin/notification/v1"
)

func (c *Client) SetClient(notificationClient notificationv1.NotificationServiceClient) {
	c.notificationClient = notificationClient
}
