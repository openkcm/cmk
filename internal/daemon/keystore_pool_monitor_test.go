package daemon_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/daemon"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

func addToKeystorePool(t *testing.T, dbRepo *sql.ResourceRepository, count int) {
	t.Helper()

	for range count {
		configBytes, err := json.Marshal(map[string]any{
			"key":        "value",
			"randomness": uuid.New().String(),
		})
		assert.NoError(t, err)

		ks := model.Keystore{
			ID:       uuid.New(),
			Provider: "test-provider",
			Config:   configBytes,
		}

		err = dbRepo.Create(t.Context(), &ks)
		assert.NoError(t, err)
	}
}

func getKeystorePoolMetric(t *testing.T, reader *metric.ManualReader) (bool, int64) {
	t.Helper()

	var metrics metricdata.ResourceMetrics
	err := reader.Collect(t.Context(), &metrics)
	assert.NoError(t, err)

	for _, scopeMetrics := range metrics.ScopeMetrics {
		for _, m := range scopeMetrics.Metrics {
			if m.Name == "keystore_pool_available" {
				gauge, ok := m.Data.(metricdata.Gauge[int64])
				assert.True(t, ok, "Expected metric data to be of type Gauge[int64]")
				if len(gauge.DataPoints) > 0 {
					assert.Len(t, gauge.DataPoints, 1)
					return true, gauge.DataPoints[0].Value
				}
			}
		}
	}

	return false, 0
}

func TestKeystorePoolMonitorCallback(t *testing.T) {
	db, _, dbConf := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	})
	dbRepo := sql.NewRepository(db)

	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)

	// Initially add 5 entries to the keystore pool before starting the monitor
	addToKeystorePool(t, dbRepo, 5)

	daemon.MonitorKeystorePoolSize(t.Context(), &config.Config{
		BaseConfig: commoncfg.BaseConfig{
			Application: commoncfg.Application{
				Name: "test-app",
			},
		},
		Database: dbConf,
		KeystorePool: config.KeystorePool{
			Interval: 5 * time.Second,
		},
	})

	t.Run("initial read", func(t *testing.T) {
		time.Sleep(1 * time.Second) // Ensure the initial entries are committed before starting the monitor

		var metrics metricdata.ResourceMetrics
		err := reader.Collect(t.Context(), &metrics)
		assert.NoError(t, err)

		// Assert that the gauge has been observed and cached
		found, value := getKeystorePoolMetric(t, reader)
		assert.True(t, found, "Expected to find keystore_pool_available metric")
		assert.Equal(t, int64(5), value, "Expected keystore_pool_available to be 5")
	})

	t.Run("previous value is returned within interval", func(t *testing.T) {
		time.Sleep(2 * time.Second)
		addToKeystorePool(t, dbRepo, 3)

		var metrics metricdata.ResourceMetrics
		err := reader.Collect(t.Context(), &metrics)
		assert.NoError(t, err)

		// Assert that the gauge still shows the old value (5) because it's within the interval
		found, value := getKeystorePoolMetric(t, reader)
		assert.True(t, found, "Expected to find keystore_pool_available metric")
		assert.Equal(t, int64(5), value, "Expected keystore_pool_available to still be 5 due to caching")
	})

	t.Run("value is refreshed after interval", func(t *testing.T) {
		time.Sleep(5 * time.Second)

		var metrics metricdata.ResourceMetrics
		err := reader.Collect(t.Context(), &metrics)
		assert.NoError(t, err)

		// Assert that the gauge now shows the updated value (8) after the interval has passed
		found, value := getKeystorePoolMetric(t, reader)
		assert.True(t, found, "Expected to find keystore_pool_available metric")
		assert.Equal(t, int64(8), value, "Expected keystore_pool_available to be 8 after refresh")
	})
}
