package auditor

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"
)

// SendCmkUnauthorizedRequestAuditLog sends an audit log for CMK Unauthorized
func (a *Auditor) SendCmkUnauthorizedRequestAuditLog(ctx context.Context, resource, action string) error {
	return a.sendEvent(ctx, func(metadata otlpaudit.EventMetadata) (plog.Logs, error) {
		return otlpaudit.NewUnauthorizedRequestEvent(metadata, resource, action)
	})
}
