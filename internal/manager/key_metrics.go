package manager

import (
	"context"

	"github.com/openkcm/common-sdk/pkg/otlp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/openkcm/cmk/internal/config"
)

const (
	pendingKeyTransitionCounterName = "pending_key_transitions_total"
	pendingKeyTransitionCounterDesc = "Number of PENDING_REGISTRATION/PENDING_CREATION key state transitions"
)

var (
	attrKeyType   = attribute.Key("key_type")
	attrFromState = attribute.Key("from_state")
	attrToState   = attribute.Key("to_state")
	attrReason    = attribute.Key("reason")
)

// KeyMetrics holds OpenTelemetry instruments for key lifecycle events.
type KeyMetrics struct {
	transitionCounter metric.Int64Counter
}

func NewKeyMetrics(cfg *config.Config) (*KeyMetrics, error) {
	meter := otel.Meter(
		cfg.Application.Name,
		metric.WithInstrumentationVersion(otel.Version()),
		metric.WithInstrumentationAttributes(otlp.CreateAttributesFrom(cfg.Application)...),
	)

	counter, err := meter.Int64Counter(
		pendingKeyTransitionCounterName,
		metric.WithDescription(pendingKeyTransitionCounterDesc),
	)
	if err != nil {
		return nil, err
	}

	return &KeyMetrics{transitionCounter: counter}, nil
}

// RecordTransition increments the transition counter with the given labels.
// keyType: "HYOK"|"BYOK"; fromState: e.g. "PENDING_REGISTRATION"; toState: e.g. "ENABLED";
// reason: "auth_success"|"timeout"|"provision_success"|"provision_error".
func (m *KeyMetrics) RecordTransition(ctx context.Context, keyType, fromState, toState, reason string) {
	if m == nil {
		return
	}
	m.transitionCounter.Add(ctx, 1,
		metric.WithAttributes(
			attrKeyType.String(keyType),
			attrFromState.String(fromState),
			attrToState.String(toState),
			attrReason.String(reason),
		),
	)
}
