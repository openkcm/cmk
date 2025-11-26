package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk/cmd/tenant-manager-cli/commands"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/log"
)

func runFuncWithSignalHandling(f func(context.Context, *config.Config) error) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create signal channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Handle signals in a separate goroutine
	go func() {
		<-sigChan
		log.Info(ctx, "Interrupt signal received, shutting down...")
		cancel()
	}()

	cfg, err := config.LoadConfig(
		commoncfg.WithPaths(
			".",
			"/etc/tenant-manager-cli",
		),
	)
	if err != nil {
		log.Error(ctx, "Failed to load config:", err)

		return 1
	}

	log.Debug(ctx, "Starting the application", slog.Any("config", cfg))

	err = f(ctx, cfg)
	if err != nil {
		log.Error(ctx, "Falied running tenant-manager-cli", err)
		return 1
	}

	return 0
}

func run(ctx context.Context, cfg *config.Config) error {
	err := logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("main").Wrapf(err, "Failed to initialise the logger")
	}

	dbCon, err := db.StartDB(ctx, cfg.Database, cfg.Provisioning, cfg.DatabaseReplicas)
	if err != nil {
		return oops.In("main").Wrapf(err, "Failed to initialise db connection")
	}

	ctlg, err := catalog.New(ctx, *cfg)
	if err != nil {
		return oops.In("main").Wrapf(err, "Failed to initialise plugin catalog")
	}

	rootCmd := setupCommands(ctx, dbCon, ctlg)

	err = rootCmd.ExecuteContext(ctx)
	if err != nil {
		return oops.In("main").Wrapf(err, "error executing command")
	}

	return nil
}

// setupCommands creates and configures all CLI commands and flags
func setupCommands(ctx context.Context, dbCon *multitenancy.DB, catalog *plugincatalog.Catalog) *cobra.Command {
	factory := commands.NewCommandFactory(dbCon, catalog)
	rootCmd := factory.NewRootCmd(ctx)

	createGroupsCmd := factory.NewCreateGroupsCmd(ctx)
	rootCmd.AddCommand(createGroupsCmd)

	createCmd := factory.NewCreateTenantCmd(ctx)
	rootCmd.AddCommand(createCmd)

	deleteTenantCmd := factory.NewDeleteTenantCmd(ctx)
	rootCmd.AddCommand(deleteTenantCmd)

	getTenantCmd := factory.NewGetTenantCmd(ctx)
	rootCmd.AddCommand(getTenantCmd)

	listTenantsCmd := factory.NewListTenantsCmd(ctx)
	rootCmd.AddCommand(listTenantsCmd)

	updateTenantCmd := factory.NewUpdateTenantCmd(ctx)
	rootCmd.AddCommand(updateTenantCmd)

	return rootCmd
}

func main() {
	exitCode := runFuncWithSignalHandling(run)
	os.Exit(exitCode)
}
