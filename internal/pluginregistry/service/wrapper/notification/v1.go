package notification

import (
	"context"
	"errors"
	"fmt"

	"buf.build/go/protovalidate"
	"github.com/openkcm/plugin-sdk/api"
	"github.com/openkcm/plugin-sdk/pkg/plugin"

	grpcnotification1 "github.com/openkcm/plugin-sdk/proto/plugin/notification/v1"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/notification"
)

var (
	ErrFailedSendNotification = errors.New("failed to send notification")
)

type V1 struct {
	plugin.Facade
	grpcnotification1.NotificationServicePluginClient
}

func (v1 *V1) Version() uint {
	return 1
}

func (v1 *V1) ServiceInfo() api.Info {
	return v1.Info
}

func (v1 *V1) Send(
	ctx context.Context,
	req *notification.SendNotificationRequest,
) (*notification.SendNotificationResponse, error) {
	in := &grpcnotification1.SendNotificationRequest{
		NotificationType: grpcnotification1.NotificationType(req.Type),
		Recipients:       req.Recipients,
		Subject:          req.Subject,
		Body:             req.Body,
	}
	if err := protovalidate.Validate(in); err != nil {
		return nil, fmt.Errorf("failed validation: %w", err)
	}

	grpcResp, err := v1.SendNotification(ctx, in)
	if err != nil {
		return nil, err
	}

	if !grpcResp.GetSuccess() {
		return nil, errs.Wrapf(ErrFailedSendNotification, grpcResp.GetMessage())
	}

	return &notification.SendNotificationResponse{}, nil
}
