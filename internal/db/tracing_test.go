package db_test

import (
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	pg "github.com/bartventer/gorm-multitenancy/postgres/v8"

	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/db/dialect"
	"github.com/openkcm/cmk/internal/db/dsn"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestWrapDialectorWithTracing(t *testing.T) {
	ctx := t.Context()
	// Get a valid DSN from test database
	_, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})
	dsnStr, err := dsn.FromDBConfig(dbCfg)
	require.NoError(t, err)

	t.Run("should return original dialector when telemetryCfg is nil", func(t *testing.T) {
		// Arrange
		originalDialector := postgres.New(postgres.Config{
			DSN: dsnStr,
		})

		// Act
		wrappedDialector, err := db.WrapDialectorWithTracing(
			ctx,
			originalDialector,
			dsnStr,
			nil,
		)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, originalDialector, wrappedDialector)
	})

	t.Run("should return original dialector when tracing is disabled", func(t *testing.T) {
		// Arrange
		originalDialector := postgres.New(postgres.Config{
			DSN: dsnStr,
		})

		telemetryCfg := &commoncfg.Telemetry{
			Traces: commoncfg.Trace{
				Enabled: false,
			},
		}

		// Act
		wrappedDialector, err := db.WrapDialectorWithTracing(
			ctx,
			originalDialector,
			dsnStr,
			telemetryCfg,
		)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, originalDialector, wrappedDialector)
	})

	t.Run("should wrap dialector when tracing is enabled", func(t *testing.T) {
		// Arrange
		originalDialector := postgres.New(postgres.Config{
			DSN: dsnStr,
		})

		telemetryCfg := &commoncfg.Telemetry{
			Traces: commoncfg.Trace{
				Enabled: true,
			},
		}

		// Act
		wrappedDialector, err := db.WrapDialectorWithTracing(
			ctx,
			originalDialector,
			dsnStr,
			telemetryCfg,
		)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, wrappedDialector)
		assert.NotEqual(t, originalDialector, wrappedDialector)

		// Verify the wrapped dialector is of the expected type
		_, isPostgresDialector := wrappedDialector.(*postgres.Dialector)
		assert.True(t, isPostgresDialector, "wrapped dialector should be a postgres dialector")
	})

	t.Run("should wrap dialector even with malformed DSN when tracing is enabled", func(t *testing.T) {
		// Arrange - otelsql.Open doesn't validate the DSN format, only registers the driver
		// Actual connection validation happens when GORM tries to connect
		malformedDSN := "host=nonexistent user=test password=test dbname=test port=99999 sslmode=disable"
		originalDialector := postgres.New(postgres.Config{
			DSN: malformedDSN,
		})

		telemetryCfg := &commoncfg.Telemetry{
			Traces: commoncfg.Trace{
				Enabled: true,
			},
		}

		// Act
		wrappedDialector, err := db.WrapDialectorWithTracing(
			ctx,
			originalDialector,
			malformedDSN,
			telemetryCfg,
		)

		// Assert - wrapping succeeds, actual connection failure would happen later in GORM
		require.NoError(t, err)
		assert.NotNil(t, wrappedDialector)
		assert.NotEqual(t, originalDialector, wrappedDialector)
	})

	t.Run("should create working database connection with tracing enabled", func(t *testing.T) {
		// Arrange
		originalDialector := postgres.New(postgres.Config{
			DSN: dsnStr,
		})

		telemetryCfg := &commoncfg.Telemetry{
			Traces: commoncfg.Trace{
				Enabled: true,
			},
		}

		// Act
		wrappedDialector, err := db.WrapDialectorWithTracing(
			ctx,
			originalDialector,
			dsnStr,
			telemetryCfg,
		)
		require.NoError(t, err)

		// Try to open a connection with the wrapped dialector
		gormDB, err := gorm.Open(wrappedDialector, &gorm.Config{})

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, gormDB)

		// Verify we can execute a simple query
		var result int
		err = gormDB.Raw("SELECT 1").Scan(&result).Error
		require.NoError(t, err)
		assert.Equal(t, 1, result)
	})

	t.Run("should preserve multitenancy dialector type and registry when tracing is enabled", func(t *testing.T) {
		// Arrange — gorm-multitenancy dialector
		originalDialector := dialect.NewFrom(dsnStr)
		_, ok := originalDialector.(*pg.Dialector)
		require.True(t, ok, "dialect.NewFrom must return a *pg.Dialector")

		telemetryCfg := &commoncfg.Telemetry{
			Traces: commoncfg.Trace{Enabled: true},
		}

		// Act
		wrappedDialector, err := db.WrapDialectorWithTracing(
			ctx,
			originalDialector,
			dsnStr,
			telemetryCfg,
		)

		// Assert — same pointer returned, type unchanged
		require.NoError(t, err)
		pgDialector, ok := wrappedDialector.(*pg.Dialector)
		require.True(t, ok, "wrapped dialector must still be a *pg.Dialector")
		assert.Same(t, originalDialector, pgDialector, "must be the same pointer, not a new instance")

		// Assert — RegisterModels still works
		err = pgDialector.RegisterModels()
		assert.NoError(t, err, "RegisterModels must still be callable after wrapping")

		// Assert — connection is functional
		gormDB, err := gorm.Open(wrappedDialector, &gorm.Config{})
		require.NoError(t, err)
		var result int
		err = gormDB.Raw("SELECT 1").Scan(&result).Error
		require.NoError(t, err)
		assert.Equal(t, 1, result)
	})
}
