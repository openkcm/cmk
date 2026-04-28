package client

import (
	"context"
	"errors"

	"github.com/openkcm/cmk/internal/log"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/notification"
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
	svc notification.Notification
}

func New(
	ctx context.Context,
	svcRegistry serviceapi.Registry,
) (*Client, error) {
	svc, err := svcRegistry.Notification()
	if err != nil {
		log.Error(ctx, "Getting notification service from registry", err)
		return nil, ErrLoadNotificationPlugin
	}

	return &Client{
		svc: svc,
	}, nil
}

func (c *Client) CreateNotification(ctx context.Context, data Data) error {
	_, err := c.svc.Send(ctx, &notification.SendNotificationRequest{
		Type:       notification.Email,
		Recipients: data.Recipients,
		Subject:    data.Subject,
		Body:       data.Body,
	})

	return err
}
