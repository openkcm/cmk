package main

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestDBMigratorRun(t *testing.T) {
	t.Run("Should fail on unsupported flags", func(t *testing.T) {
		_, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})
		cfg := &config.Config{Database: dbCfg}

		flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
		migrationType = flag.String("type", "error", "Set the migration type. Can be data/schema")

		err := run(t.Context(), cfg)
		assert.ErrorIs(t, err, db.ErrUnsupportedMigration)
	})
}
