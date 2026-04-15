package db_test

import (
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/db/dsn"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestWrapDialectorWithTracing(t *testing.T) {
	// Get a valid DSN from test database
	_, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})
	dsnStr, err := dsn.FromDBConfig(dbCfg)
	require.NoError(t, err)

	t.Run("should return original dialector when telemetryCfg is nil", func(t *testing.T) {
		// Arrange
		originalDialector := postgres.New(postgres.Config{
			DSN:                  dsnStr,
			PreferSimpleProtocol: true,
		})

		// Act
		wrappedDialector, err := db.WrapDialectorWithTracing(
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
			DSN:                  dsnStr,
			PreferSimpleProtocol: true,
		})

		telemetryCfg := &commoncfg.Telemetry{
			Traces: commoncfg.Trace{
				Enabled: false,
			},
		}

		// Act
		wrappedDialector, err := db.WrapDialectorWithTracing(
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
			DSN:                  dsnStr,
			PreferSimpleProtocol: true,
		})

		telemetryCfg := &commoncfg.Telemetry{
			Traces: commoncfg.Trace{
				Enabled: true,
			},
		}

		// Act
		wrappedDialector, err := db.WrapDialectorWithTracing(
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
			DSN:                  malformedDSN,
			PreferSimpleProtocol: true,
		})

		telemetryCfg := &commoncfg.Telemetry{
			Traces: commoncfg.Trace{
				Enabled: true,
			},
		}

		// Act
		wrappedDialector, err := db.WrapDialectorWithTracing(
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
			DSN:                  dsnStr,
			PreferSimpleProtocol: true,
		})

		telemetryCfg := &commoncfg.Telemetry{
			Traces: commoncfg.Trace{
				Enabled: true,
			},
		}

		// Act
		wrappedDialector, err := db.WrapDialectorWithTracing(
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
}
