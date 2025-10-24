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

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/tenant-manager/tenant-cli/cmd"
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

	cfg, err := loadConfig()
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

	rootCmd := setupCommands(ctx, dbCon)

	err = rootCmd.ExecuteContext(ctx)
	if err != nil {
		return oops.In("main").Wrapf(err, "error executing command")
	}

	return nil
}

// setupCommands creates and configures all CLI commands and flags
func setupCommands(ctx context.Context, dbCon *multitenancy.DB) *cobra.Command {
	var (
		id, region, status string
		sleep              bool
	)

	factory := cmd.NewCommandFactory(dbCon)
	rootCmd := factory.NewRootCmd(ctx)
	rootCmd.PersistentFlags().BoolVar(&sleep, "sleep", false, "Enable sleep mode")

	createGroupsCmd := factory.NewCreateGroupsCmd(ctx)
	createGroupsCmd.Flags().StringVarP(&id, "id", "i", "", "Tenant id")
	rootCmd.AddCommand(createGroupsCmd)

	createCmd := factory.NewCreateTenantCmd(ctx)
	createCmd.Flags().StringVarP(&id, "id", "i", "", "Tenant id")
	createCmd.Flags().StringVarP(&region, "region", "r", "", "Tenant region")
	createCmd.Flags().StringVarP(&status, "status", "s", "", "Tenant status")
	rootCmd.AddCommand(createCmd)

	deleteTenantCmd := factory.NewDeleteTenantCmd(ctx)
	deleteTenantCmd.Flags().StringVarP(&id, "id", "i", "", "Tenant id")
	rootCmd.AddCommand(deleteTenantCmd)

	getTenantCmd := factory.NewGetTenantCmd(ctx)
	getTenantCmd.Flags().StringVarP(&id, "id", "i", "", "Tenant id")
	rootCmd.AddCommand(getTenantCmd)

	listTenantsCmd := factory.NewListTenantsCmd(ctx)
	rootCmd.AddCommand(listTenantsCmd)

	updateTenantCmd := factory.NewUpdateTenantCmd(ctx)
	updateTenantCmd.Flags().StringVarP(&id, "id", "i", "", "Tenant id")
	updateTenantCmd.Flags().StringVarP(&region, "region", "r", "", "Tenant region")
	updateTenantCmd.Flags().StringVarP(&status, "status", "s", "", "Tenant status")
	rootCmd.AddCommand(updateTenantCmd)

	return rootCmd
}

func loadConfig() (*config.Config, error) {
	cfg := &config.Config{}

	loader := commoncfg.NewLoader(
		cfg,
		commoncfg.WithPaths(
			".",
			"/etc/tenant-manager-cli",
		),
		commoncfg.WithEnvOverride(constants.APIName),
	)

	err := loader.LoadConfig()
	if err != nil {
		return nil, oops.In("main").Wrapf(err, "failed to load config")
	}

	return cfg, nil
}

func main() {
	exitCode := runFuncWithSignalHandling(run)
	os.Exit(exitCode)
}
