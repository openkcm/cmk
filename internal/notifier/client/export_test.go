package client

import (
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/notification"
)

func (c *Client) SetService(svc notification.Notification) {
	c.svc = svc
}
