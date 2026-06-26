package main

import (
	"context"
	"flag"
	"os"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/cmd/tenant-manager-cli/commands"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/log"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

var (
	gracefulShutdownSec     = flag.Int64("graceful-shutdown", 1, "graceful shutdown seconds")
	gracefulShutdownMessage = flag.String(
		"graceful-shutdown-message", "Graceful shutdown in %d seconds",
		"graceful shutdown message",
	)
)

func run(ctx context.Context, cfg *config.Config) error {
	err := logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("main").Wrapf(err, "Failed to initialise the logger")
	}

	dbCon, err := db.StartDBConnection(ctx, cfg.Database, cfg.DatabaseReplicas, &cfg.Telemetry)
	if err != nil {
		return oops.In("main").Wrapf(err, "Failed to initialise db connection")
	}

	svcRegistry, err := cmkpluginregistry.New(ctx, cfg)
	if err != nil {
		return oops.In("main").Wrapf(err, "Failed to initialise plugin catalog")
	}

	rootCmd, err := setupCommands(ctx, cfg, dbCon, svcRegistry)
	if err != nil {
		return oops.In("main").Wrapf(err, "Failed to initialise commands")
	}

	err = rootCmd.ExecuteContext(ctx)
	if err != nil {
		return oops.In("main").Wrapf(err, "error executing command")
	}

	return nil
}

// setupCommands creates and configures all CLI commands and flags
func setupCommands(
	ctx context.Context,
	cfg *config.Config,
	dbCon *multitenancy.DB,
	svcRegistry serviceapi.Registry,
) (*cobra.Command, error) {
	factory, err := commands.NewCommandFactory(ctx, cfg, dbCon, svcRegistry)
	if err != nil {
		return nil, err
	}

	ctx, err = cmkcontext.InjectInternalUserData(ctx, constants.InternalTenantCLIRole)
	if err != nil {
		return nil, err
	}

	rootCmd := factory.NewRootCmd(ctx, cfg)

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

	return rootCmd, nil
}

func main() {
	flag.Parse()

	exitCode := cmd.RunFuncWithSignalHandling(run, cmd.RunFlags{
		GracefulShutdownSec:     *gracefulShutdownSec,
		GracefulShutdownMessage: *gracefulShutdownMessage,
		Env:                     "TENANT_MANAGER_CLI",
		LoadOptions: []commoncfg.Option{
			commoncfg.WithPaths(
				constants.DefaultConfigPath1,
				constants.DefaultConfigPath2,
				".",
				"/etc/tenant-manager-cli",
			),
		},
	})
	os.Exit(exitCode)
}
