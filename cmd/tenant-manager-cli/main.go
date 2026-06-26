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

	"github.tools.sap/kms/cmk/cmd/tenant-manager-cli/commands"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/db"
	cmkpluginregistry "github.tools.sap/kms/cmk/internal/pluginregistry"
	serviceapi "github.tools.sap/kms/cmk/internal/pluginregistry/service/api"
	cliUtils "github.tools.sap/kms/cmk/utils/cli"
	"github.tools.sap/kms/cmk/utils/cmd"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
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

	root := &cobra.Command{
		Use:   "tm",
		Short: "Tenant Manager CLI Application",
		Long: "Tenant Manager is a simple CLI tool to manage tenants, supporting: creating tenant, " +
			"creating tenant with groups, " +
			"creating groups, " +
			"updating of region and status field on a tenant entity in public table, " +
			"updating of group names, " +
			"changing any field value in any table of a tenant schema.",

		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			ctx := context.WithValue(cmd.Context(), commands.TenantManagerFactoryKey, factory)
			cmd.SetContext(ctx)
		},
	}

	root.AddCommand(commands.NewCreateTenantCmd())
	root.AddCommand(commands.NewDeleteTenantCmd())
	root.AddCommand(commands.NewGetTenantCmd())
	root.AddCommand(commands.NewListTenantsCmd())
	root.AddCommand(commands.NewUpdateTenantCmd())
	root.AddCommand(cliUtils.NewSleep(cfg))

	return root, nil
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
