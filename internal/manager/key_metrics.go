package manager

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

const (
	metricPendingDuration = "cmk_pending_key_duration_seconds"
	metricPendingFailures = "cmk_pending_key_failures_total"
)

type pendingKeyMetrics struct {
	duration metric.Float64Histogram
	failures metric.Int64Counter
}

func newPendingKeyMetrics(meterName string) (*pendingKeyMetrics, error) {
	meter := otel.Meter(meterName)

	duration, err := meter.Float64Histogram(
		metricPendingDuration,
		metric.WithDescription("Duration a key spent in a pending state before terminal transition"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	failures, err := meter.Int64Counter(
		metricPendingFailures,
		metric.WithDescription("Number of pending keys that transitioned to a failure state (ERROR or FORBIDDEN)"),
	)
	if err != nil {
		return nil, err
	}

	return &pendingKeyMetrics{duration: duration, failures: failures}, nil
}

// recordTerminal records metrics when a pending key reaches a terminal state.
// It must be called after the key's State has been updated to the terminal value.
// failed should be true for ERROR and FORBIDDEN states.
func (m *pendingKeyMetrics) recordTerminal(ctx context.Context, key *model.Key, failed bool) {
	elapsed := time.Since(key.CreatedAt).Seconds()

	keyType := string(key.KeyType)
	region := key.Region
	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		log.Debug(ctx, "pending key metrics: could not extract tenant ID", log.ErrorAttr(err))
	}

	attrs := metric.WithAttributes(
		attribute.String("key_type", keyType),
		attribute.String("tenant", tenantID),
		attribute.String("region", region),
		attribute.String("terminal_state", string(key.State)),
	)

	m.duration.Record(ctx, elapsed, attrs)
	if failed {
		m.failures.Add(ctx, 1, attrs)
	}
}

