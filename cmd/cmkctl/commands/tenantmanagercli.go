package commands

import (
	"context"
	"fmt"
	"log/slog"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	tmcommands "github.tools.sap/kms/cmk/cmd/tenant-manager-cli/commands"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/db"
	"github.tools.sap/kms/cmk/internal/log"
	cmkpluginregistry "github.tools.sap/kms/cmk/internal/pluginregistry"
	serviceapi "github.tools.sap/kms/cmk/internal/pluginregistry/service/api"
	runcmd "github.tools.sap/kms/cmk/utils/cmd"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

func NewTenantManagerCLI() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenant",
		Short: "Tenant management CLI",
		Long:  `Manage tenants: create, delete, list, and update tenant configurations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			exitCode := runcmd.RunFuncWithSignalHandling(runTenantManagerCLI, runcmd.RunFlags{
				GracefulShutdownSec:     gracefulShutdownSec,
				GracefulShutdownMessage: gracefulShutdownMessage,
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
			if exitCode != 0 {
				return fmt.Errorf("tenant-manager-cli exited with code %d", exitCode)
			}
			return nil
		},
	}

	cmd.Flags().Int64Var(&gracefulShutdownSec, "graceful-shutdown", 1, "graceful shutdown seconds")
	cmd.Flags().StringVar(&gracefulShutdownMessage, "graceful-shutdown-message",
		"Graceful shutdown in %d seconds", "graceful shutdown message")

	return cmd
}

func runTenantManagerCLI(ctx context.Context, cfg *config.Config) error {
	err := logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("tenant-manager-cli").Wrapf(err, "Failed to initialise the logger")
	}

	log.Debug(ctx, "Starting tenant-manager-cli", slog.Any("config", cfg))

	dbCon, err := db.StartDBConnection(ctx, cfg.Database, cfg.DatabaseReplicas, &cfg.Telemetry)
	if err != nil {
		return oops.In("tenant-manager-cli").Wrapf(err, "Failed to initialise db connection")
	}

	svcRegistry, err := cmkpluginregistry.New(ctx, cfg)
	if err != nil {
		return oops.In("tenant-manager-cli").Wrapf(err, "Failed to initialise plugin catalog")
	}

	rootCmd, err := setupTenantCommands(ctx, cfg, dbCon, svcRegistry)
	if err != nil {
		return oops.In("tenant-manager-cli").Wrapf(err, "Failed to initialise commands")
	}

	err = rootCmd.ExecuteContext(ctx)
	if err != nil {
		return oops.In("tenant-manager-cli").Wrapf(err, "error executing command")
	}

	return nil
}

// setupTenantCommands creates and configures all tenant CLI commands and flags
func setupTenantCommands(
	ctx context.Context,
	cfg *config.Config,
	dbCon *multitenancy.DB,
	svcRegistry serviceapi.Registry,
) (*cobra.Command, error) {
	factory, err := tmcommands.NewCommandFactory(ctx, cfg, dbCon, svcRegistry)
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
