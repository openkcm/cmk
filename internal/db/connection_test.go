//go:build !unit

package db_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/db"
	"github.tools.sap/kms/cmk/internal/testutils"
)

var errForced = errors.New("forced error")

type errorPlugin struct{}

func (errorPlugin) Name() string {
	return "errorPlugin"
}

func (errorPlugin) Initialize(_ *gorm.DB) error {
	return errForced
}

// TestStartDBConnectionPlugins tests the StartDBConnectionPlugins function
func TestStartDBConnectionPlugins(t *testing.T) {
	t.Run("should error on start db connection with invalid config", func(t *testing.T) {
		dbConn, err := db.StartDBConnectionPlugins(
			t.Context(),
			config.Database{},
			[]config.Database{},
			map[string]gorm.Plugin{"error": errorPlugin{}},
		)

		require.Error(t, err)
		require.Nil(t, dbConn)
	})

	t.Run("should start db connection with replicas", func(t *testing.T) {
		dbConn, err := db.StartDBConnectionPlugins(
			t.Context(),
			testutils.TestDB,
			[]config.Database{testutils.TestDB},
			map[string]gorm.Plugin{},
		)
		require.NoError(t, err)
		require.NotNil(t, dbConn)
	})

	t.Run("should error start db connection with replicas", func(t *testing.T) {
		dbConn, err := db.StartDBConnectionPlugins(
			t.Context(),
			testutils.TestDB,
			[]config.Database{{}},
			map[string]gorm.Plugin{},
		)
		require.ErrorIs(t, err, db.ErrLoadingReplicaDialectors)
		require.Nil(t, dbConn)
	})
}

// TestStartDBConnection_postgres - tests the StartDBConnection function.
func TestStartDBConnection(t *testing.T) {
	t.Run("should start db connection when config is valid", func(t *testing.T) {
		dbConn, err := db.StartDBConnection(
			t.Context(),
			testutils.TestDB,
			[]config.Database{},
		)

		require.NoError(t, err)
		require.NotNil(t, dbConn)
	})
}
