package main

import (
	"context"
	"flag"
	"os"

	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/utils/cmd"
)

const (
	defaultGracefulShutdown = 1
	defaultTarget           = "all"
	defaultType             = "schema"
	targetOptions           = "shared, all, or tenant"
	typeOptions             = "data or schema"
)

var (
	gracefulShutdownSec     = flag.Int64("graceful-shutdown", defaultGracefulShutdown, "graceful shutdown seconds")
	gracefulShutdownMessage = flag.String(
		"graceful-shutdown-message",
		"Graceful shutdown in %d seconds",
		"graceful shutdown message",
	)
	version       = flag.Int64("version", 0, "run migration until targeted version")
	rollback      = flag.Bool("r", false, "run down migrations (rollback)")
	target        = flag.String("target", defaultTarget, "migration target ("+targetOptions+")")
	migrationType = flag.String("type", defaultType, "migration type ("+typeOptions+")")
)

func run(ctx context.Context, cfg *config.Config) error {
	err := logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("main").
			Wrapf(err, "Failed to initialise the logger")
	}

	dbCon, err := db.StartDBConnection(ctx, cfg.Database, cfg.DatabaseReplicas)
	if err != nil {
		return err
	}

	r := sql.NewRepository(dbCon)
	m, err := db.NewMigrator(r, cfg)
	if err != nil {
		return err
	}

	req := db.Migration{
		Downgrade: *rollback,
		Type:      db.MigrationType(*migrationType),
		Target:    db.MigrationTarget(*target),
	}

	if *version != 0 {
		err = m.MigrateTo(ctx, req, *version)
	} else {
		err = m.MigrateToLatest(ctx, req)
	}
	if err != nil {
		return err
	}

	return nil
}

// main is the entry point for the application. It is intentionally kept small
// because it is hard to test, which would lower test coverage.
func main() {
	flag.Parse()

	exitCode := cmd.RunFuncWithSignalHandling(run, cmd.RunFlags{
		GracefulShutdownSec:     *gracefulShutdownSec,
		GracefulShutdownMessage: *gracefulShutdownMessage,
		Env:                     "DB_MIGRATOR",
	})
	os.Exit(exitCode)
}
