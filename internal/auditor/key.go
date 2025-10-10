package auditor

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"
)

// SendCmkCreateAuditLog sends an audit log for CMK creation
func (a *Auditor) SendCmkCreateAuditLog(ctx context.Context, cmkID string) error {
	return a.sendEvent(ctx, func(metadata otlpaudit.EventMetadata) (plog.Logs, error) {
		return otlpaudit.NewCmkCreateEvent(metadata, cmkID)
	})
}

// SendCmkDeleteAuditLog sends an audit log for CMK deletion
func (a *Auditor) SendCmkDeleteAuditLog(ctx context.Context, cmkID string) error {
	return a.sendEvent(ctx, func(metadata otlpaudit.EventMetadata) (plog.Logs, error) {
		return otlpaudit.NewCmkDeleteEvent(metadata, cmkID)
	})
}

// SendCmkEnableAuditLog sends an audit log for CMK enabling
func (a *Auditor) SendCmkEnableAuditLog(ctx context.Context, cmkID string) error {
	return a.sendEvent(ctx, func(metadata otlpaudit.EventMetadata) (plog.Logs, error) {
		return otlpaudit.NewCmkEnableEvent(metadata, cmkID)
	})
}

// SendCmkDisableAuditLog sends an audit log for CMK disabling
func (a *Auditor) SendCmkDisableAuditLog(ctx context.Context, cmkID string) error {
	return a.sendEvent(ctx, func(metadata otlpaudit.EventMetadata) (plog.Logs, error) {
		return otlpaudit.NewCmkDisableEvent(metadata, cmkID)
	})
}

// SendCmkRotateAuditLog sends an audit log for CMK rotation
func (a *Auditor) SendCmkRotateAuditLog(ctx context.Context, cmkID string) error {
	return a.sendEvent(ctx, func(metadata otlpaudit.EventMetadata) (plog.Logs, error) {
		return otlpaudit.NewCmkRotateEvent(metadata, cmkID)
	})
}
