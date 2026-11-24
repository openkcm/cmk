package testutils

import (
	"sync/atomic"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

// FailingMeterProvider fails Orbital and Redis metric creation without affecting other meters.
type FailingMeterProvider struct {
	noop.MeterProvider

	Err    error
	Called atomic.Bool
}

func (p *FailingMeterProvider) Meter(name string, opts ...metric.MeterOption) metric.Meter {
	if name != "github.com/openkcm/orbital" && name != "github.com/redis/go-redis/extra/redisotel" {
		return p.MeterProvider.Meter(name, opts...)
	}
	return failingMeter{provider: p}
}

type failingMeter struct {
	noop.Meter

	provider *FailingMeterProvider
}

func (m failingMeter) Float64Histogram(_ string, _ ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	m.provider.Called.Store(true)
	return nil, m.provider.Err
}

func (m failingMeter) Int64ObservableUpDownCounter(
	_ string, _ ...metric.Int64ObservableUpDownCounterOption,
) (metric.Int64ObservableUpDownCounter, error) {
	m.provider.Called.Store(true)
	return nil, m.provider.Err
}
