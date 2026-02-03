package auditor

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"
)

// SendCmkTenantDeleteAuditLog sends an audit log for CMK tenant deletion.
func (a *Auditor) SendCmkTenantDeleteAuditLog(ctx context.Context, tenantID string) error {
	return a.sendEvent(ctx, func(metadata otlpaudit.EventMetadata) (plog.Logs, error) {
		return otlpaudit.NewCmkTenantDeleteEvent(metadata, tenantID)
	})
}
