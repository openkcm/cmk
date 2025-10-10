package auditor

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"
)

func (a *Auditor) GetEventMetadata(ctx context.Context) (otlpaudit.EventMetadata, error) {
	return a.getEventMetadata(ctx)
}

func (a *Auditor) SendEvent(ctx context.Context, createEventFn func(otlpaudit.EventMetadata) (plog.Logs, error)) error {
	return a.sendEvent(ctx, createEventFn)
}
