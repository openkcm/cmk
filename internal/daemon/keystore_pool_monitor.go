package daemon

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/openkcm/common-sdk/pkg/otlp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/repo/sql"
)

const (
	labelKeystore = "keystore"
	gaugeName     = "keystore_pool_available"
	gaugeDesc     = "The number of keystore entries in the pool"
)

var ErrInvalidKeystorePoolInterval = errors.New("invalid keystore pool interval, must be > 0")

type KeystorePoolMonitor struct {
	meter metric.Meter
	gauge metric.Int64Gauge

	interval time.Duration

	cfg  *config.Config
	pool *manager.Pool
}

func NewKeystorePoolMonitorFromCfg(cfg *config.Config) (*KeystorePoolMonitor, error) {
	if cfg.KeystorePool.Interval <= 0 {
		return nil, ErrInvalidKeystorePoolInterval
	}

	meter := otel.Meter(
		cfg.Application.Name,
		metric.WithInstrumentationVersion(otel.Version()),
		metric.WithInstrumentationAttributes(otlp.CreateAttributesFrom(cfg.Application)...),
	)

	gauge, err := meter.Int64Gauge(gaugeName, metric.WithDescription(gaugeDesc))
	if err != nil {
		return nil, err
	}

	return &KeystorePoolMonitor{
		meter:    meter,
		gauge:    gauge,
		cfg:      cfg,
		interval: cfg.KeystorePool.Interval,
	}, nil
}

func (m *KeystorePoolMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	m.record(ctx) // Record immediately on start

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.record(ctx)
		}
	}
}

func (m *KeystorePoolMonitor) getPool(ctx context.Context) (*manager.Pool, error) {
	if m.pool != nil {
		return m.pool, nil
	}

	dbCon, err := db.StartDBConnection(ctx, m.cfg.Database, m.cfg.DatabaseReplicas, &m.cfg.Telemetry)
	if err != nil {
		return nil, err
	}

	m.pool = manager.NewPool(sql.NewRepository(dbCon))
	return m.pool, nil
}

func (m *KeystorePoolMonitor) record(ctx context.Context) {
	pool, err := m.getPool(ctx)
	if err != nil {
		log.Error(ctx, "failed to initialize keystore pool", err)
		return
	}

	count, err := pool.Count(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		log.Error(ctx, "failed to get keystore pool size", err)
		return
	}

	m.gauge.Record(ctx, int64(count))

	log.Debug(ctx, "keystore pool size", slog.Int("size", count))
}

func MonitorKeystorePoolSize(ctx context.Context, cfg *config.Config) {
	monitor, err := NewKeystorePoolMonitorFromCfg(cfg)
	if err != nil {
		log.Error(ctx, "failed to initialize keystore pool monitor", err)
		return
	}

	log.Debug(ctx, "Starting keystore pool size metric loop")
	go monitor.Start(ctx)
}
