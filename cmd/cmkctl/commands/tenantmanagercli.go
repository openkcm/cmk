package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	tmcommands "github.tools.sap/kms/cmk/cmd/tenant-manager-cli/commands"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/db"
	"github.tools.sap/kms/cmk/internal/log"
	cmkpluginregistry "github.tools.sap/kms/cmk/internal/pluginregistry"
	serviceapi "github.tools.sap/kms/cmk/internal/pluginregistry/service/api"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

func NewTenantManagerCLI() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenant",
		Short: "Tenant management CLI",
		Long:  `Manage tenants: create, delete, list, and update tenant configurations.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			cfg, dbCon, svcRegistry, err := initializeTenantManager(ctx)
			if err != nil {
				return err
			}

			// Inject internal user data
			ctx, err = cmkcontext.InjectInternalUserData(ctx, constants.InternalTenantCLIRole)
			if err != nil {
				return fmt.Errorf("failed to inject internal user data: %w", err)
			}

			// Create command factory
			factory, err := tmcommands.NewCommandFactory(ctx, cfg, dbCon, svcRegistry)
			if err != nil {
				return fmt.Errorf("failed to create command factory: %w", err)
			}

			// Store factory in context using the shared context key
			ctx = context.WithValue(ctx, tmcommands.TenantManagerFactoryKey, *factory)
			cmd.SetContext(ctx)

			return nil
		},
	}

	// Add subcommands - they retrieve factory from context
	cmd.AddCommand(tmcommands.NewCreateTenantCmd())
	cmd.AddCommand(tmcommands.NewDeleteTenantCmd())
	cmd.AddCommand(tmcommands.NewGetTenantCmd())
	cmd.AddCommand(tmcommands.NewListTenantsCmd())
	cmd.AddCommand(tmcommands.NewUpdateTenantCmd())

	return cmd
}

func initializeTenantManager(ctx context.Context) (
	*config.Config,
	*multitenancy.DB,
	serviceapi.Registry,
	error,
) {
	cfg, err := config.LoadConfig(
		commoncfg.WithPaths(
			constants.DefaultConfigPath1,
			constants.DefaultConfigPath2,
			".",
			"/etc/tenant-manager-cli",
		),
		commoncfg.WithEnvOverride("TENANT_MANAGER_CLI"),
	)
	if err != nil {
		log.Error(ctx, "Failed to load config:", err)
		return nil, nil, nil, err
	}

	err = logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return nil, nil, nil, oops.In("tenant-manager-cli").Wrapf(err, "Failed to initialise the logger")
	}

	log.Debug(ctx, "Starting tenant-manager-cli", slog.Any("config", cfg))

	dbCon, err := db.StartDBConnection(ctx, cfg.Database, cfg.DatabaseReplicas, &cfg.Telemetry)
	if err != nil {
		return nil, nil, nil, oops.In("tenant-manager-cli").Wrapf(err, "Failed to initialise db connection")
	}

	svcRegistry, err := cmkpluginregistry.New(ctx, cfg)
	if err != nil {
		return nil, nil, nil, oops.In("tenant-manager-cli").Wrapf(err, "Failed to initialise plugin catalog")
	}

	return cfg, dbCon, svcRegistry, nil
}
