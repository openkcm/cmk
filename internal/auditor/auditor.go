package auditor

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/plog"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

var (
	ErrCreateEvent         = errors.New("failed to create audit event")
	ErrCreateEventMetadata = errors.New("failed to create event metadata")
	ErrSendEvent           = errors.New("failed to send audit event")
	ErrNilAuditor          = errors.New("auditor is nil")
)

// AuditLogger interface for easier testing and dependency injection
type AuditLogger interface {
	SendEvent(ctx context.Context, logs plog.Logs) error
}

// Auditor handles audit logging for CMK operations
type Auditor struct {
	auditLogger AuditLogger
}

// New creates a new Auditor instance
func New(ctx context.Context, config *config.Config) *Auditor {
	auditLogger, err := otlpaudit.NewLogger(&config.Audit)
	if err != nil {
		log.Error(ctx, "failed to create audit logger", err)
	}

	return &Auditor{
		auditLogger: auditLogger,
	}
}

// getEventMetadata extracts common metadata from context
func (a *Auditor) getEventMetadata(ctx context.Context) (otlpaudit.EventMetadata, error) {
	if a == nil {
		return nil, ErrNilAuditor
	}

	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return nil, errs.Wrap(ErrCreateEventMetadata, err)
	}

	requestID, err := cmkcontext.GetRequestID(ctx)
	if err != nil {
		return nil, err
	}

	// Currently, user context is not implemented, so keep it as a placeholder.
	userInitID := "user-init-id-not-set"

	eventMetadata, err := otlpaudit.NewEventMetadata(userInitID, tenantID, requestID)
	if err != nil {
		return nil, errs.Wrap(ErrCreateEventMetadata, err)
	}

	return eventMetadata, nil
}

// sendEvent is a common helper for sending audit events with error handling
func (a *Auditor) sendEvent(ctx context.Context, createEventFn func(otlpaudit.EventMetadata) (plog.Logs, error)) error {
	if a == nil {
		return ErrNilAuditor
	}

	if a.auditLogger == nil {
		log.Warn(ctx, "audit logger not available, skipping audit event")

		return nil
	}

	metadata, err := a.getEventMetadata(ctx)
	if err != nil {
		return err
	}

	logs, err := createEventFn(metadata)
	if err != nil {
		return errs.Wrap(ErrCreateEvent, err)
	}

	err = a.auditLogger.SendEvent(ctx, logs)
	if err != nil {
		return errs.Wrap(ErrSendEvent, err)
	}

	return nil
}
