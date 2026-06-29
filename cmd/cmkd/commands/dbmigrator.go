package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/repo/sql"
	runcmd "github.com/openkcm/cmk/utils/cmd"
)

var (
	version       int64
	rollback      bool
	target        string
	migrationType string
)

func NewDBMigrator() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db-migrator",
		Short: "Run database migrations",
		Long:  `Runs database schema and data migrations using Goose.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			exitCode := runcmd.RunFuncWithSignalHandling(runDBMigrator, runcmd.RunFlags{
				GracefulShutdownSec:     gracefulShutdownSec,
				GracefulShutdownMessage: gracefulShutdownMessage,
				Env:                     "DB_MIGRATOR",
			})
			if exitCode != 0 {
				return fmt.Errorf("%w: db-migrator exited with code %d", ErrNonZeroExit, exitCode)
			}
			return nil
		},
	}

	cmd.Flags().Int64Var(&gracefulShutdownSec, "graceful-shutdown", 1, "graceful shutdown seconds")
	cmd.Flags().StringVar(&gracefulShutdownMessage, "graceful-shutdown-message",
		"Graceful shutdown in %d seconds", "graceful shutdown message")
	cmd.Flags().Int64Var(&version, "version", 0, "run migration until targeted version")
	cmd.Flags().BoolVarP(&rollback, "rollback", "r", false, "run down migrations (rollback)")
	cmd.Flags().StringVar(&target, "target", "all", "migration target (shared, all, or tenant)")
	cmd.Flags().StringVar(&migrationType, "type", "schema", "migration type (data or schema)")

	return cmd
}

func runDBMigrator(ctx context.Context, cfg *config.Config) error {
	err := logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("db-migrator").Wrapf(err, "Failed to initialise the logger")
	}

	dbCon, err := db.StartDBConnection(ctx, cfg.Database, cfg.DatabaseReplicas, &cfg.Telemetry)
	if err != nil {
		return err
	}

	r := sql.NewRepository(dbCon)
	m, err := db.NewMigrator(r, cfg)
	if err != nil {
		return err
	}

	req := db.Migration{
		Downgrade: rollback,
		Type:      db.MigrationType(migrationType),
		Target:    db.MigrationTarget(target),
	}

	var res db.MigrationResultMap
	if version != 0 {
		res, err = m.MigrateTo(ctx, req, version)
	} else {
		res, err = m.MigrateToLatest(ctx, req)
	}
	if err != nil {
		return err
	}

	for k, v := range res {
		for _, migration := range v {
			log.Info(ctx, "Migration Result", slog.String("Schema", k), slog.String("Result", migration.String()))
		}
	}

	return nil
}
