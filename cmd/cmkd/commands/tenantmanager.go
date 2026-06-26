package commands

import (
	"context"
	"fmt"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/openkcm/orbital/codec"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	"github.tools.sap/kms/cmk/internal/auditor"
	"github.tools.sap/kms/cmk/internal/authz"
	authz_loader "github.tools.sap/kms/cmk/internal/authz/loader"
	authz_repo "github.tools.sap/kms/cmk/internal/authz/repo"
	"github.tools.sap/kms/cmk/internal/clients"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/db"
	eventprocessor "github.tools.sap/kms/cmk/internal/event-processor"
	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/internal/operator"
	cmkpluginregistry "github.tools.sap/kms/cmk/internal/pluginregistry"
	"github.tools.sap/kms/cmk/internal/repo"
	"github.tools.sap/kms/cmk/internal/repo/sql"
	runcmd "github.tools.sap/kms/cmk/utils/cmd"
	statusserver "github.tools.sap/kms/cmk/utils/status_server"
)

const logDomain = "tenant-manager"

func NewTenantManager() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenant-manager",
		Short: "Start the tenant manager",
		Long:  `Starts the tenant manager that handles tenant lifecycle events via AMQP.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			exitCode := runcmd.RunFuncWithSignalHandling(runTenantManager, runcmd.RunFlags{
				GracefulShutdownSec:     gracefulShutdownSec,
				GracefulShutdownMessage: gracefulShutdownMessage,
				Env:                     "TENANT_MANAGER",
				LoadOptions: []commoncfg.Option{
					commoncfg.WithPaths(
						"/etc/tenant-manager",
						".",
					),
				},
			})
			if exitCode != 0 {
				return fmt.Errorf("tenant-manager exited with code %d", exitCode)
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&gracefulShutdownSec, "graceful-shutdown", 1, "graceful shutdown seconds")
	cmd.Flags().StringVar(&gracefulShutdownMessage, "graceful-shutdown-message",
		"Graceful shutdown in %d seconds", "graceful shutdown message")

	return cmd
}

func runTenantManager(ctx context.Context, cfg *config.Config) error {
	err := validateConfig(cfg)
	if err != nil {
		return err
	}

	err = initializeLoggerAndTelemetry(ctx, cfg)
	if err != nil {
		return err
	}

	statusserver.StartStatusServer(ctx, cfg)

	dbConn, err := db.StartDB(ctx, cfg)
	if err != nil {
		return oops.In(logDomain).Wrapf(err, "Failed to start the database connection")
	}

	target, err := createAMQPClient(ctx, cfg)
	if err != nil {
		return err
	}

	clients, err := validateAndGetClients(cfg)
	if err != nil {
		return err
	}

	svcRegistry, err := cmkpluginregistry.New(ctx, cfg)
	if err != nil {
		return err
	}

	r := sql.NewRepository(dbConn)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(ctx, r, cfg)
	if authzRepoLoader.AuthzHandler == nil {
		return oops.In(logDomain).Errorf("failed to create authz loader")
	}

	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	tenantManager, err := createTenantManager(ctx, authzRepoLoader, authzRepo, clients, svcRegistry, cfg)
	if err != nil {
		return err
	}

	groupManager := manager.NewGroupManager(authzRepo, svcRegistry,
		manager.NewUserManager(authzRepo, auditor.New(ctx, cfg)))

	op, err := operator.NewTenantOperator(dbConn, cfg, target, clients, tenantManager, groupManager, authzRepo)
	if err != nil {
		return oops.In(logDomain).Wrapf(err, "Failed to run operator")
	}

	return op.RunOperator(ctx)
}

func createTenantManager(
	ctx context.Context,
	authzLoader *authz_loader.AuthzLoader[authz.RepoResourceType, authz.RepoAction],
	r repo.Repo,
	clients clients.Factory,
	svcRegistry *cmkpluginregistry.Registry,
	cfg *config.Config,
) (manager.Tenant, error) {
	cmkAuditor := auditor.New(ctx, cfg)

	eventFactory, err := eventprocessor.NewEventFactory(ctx, cfg, r)
	if err != nil {
		return nil, err
	}

	cm := manager.NewCertificateManager(ctx, r, svcRegistry, cfg)
	um := manager.NewUserManager(r, cmkAuditor)
	tagm := manager.NewTagManager(r)
	kcm := manager.NewKeyConfigManager(r, cm, um, tagm, cmkAuditor, eventFactory, cfg)

	sys := manager.NewSystemManager(ctx, r, authzLoader, clients, eventFactory, svcRegistry, cfg, kcm, um)
	km := manager.NewKeyManager(r, svcRegistry, manager.NewTenantConfigManager(r, svcRegistry, cfg), kcm, um, cm, eventFactory, cmkAuditor)

	migrator, err := db.NewMigrator(r, cfg)
	if err != nil {
		return nil, err
	}

	return manager.NewTenantManager(r, sys, km, um, cmkAuditor, migrator), nil
}

func initializeLoggerAndTelemetry(ctx context.Context, cfg *config.Config) error {
	err := logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In(logDomain).Wrapf(err, "Failed to initialise the logger")
	}

	err = otlp.Init(ctx, &cfg.Application, &cfg.Telemetry, &cfg.Logger)
	if err != nil {
		return oops.In(logDomain).Wrapf(err, "Failed to load the telemetry")
	}

	return nil
}

func createAMQPClient(ctx context.Context, cfg *config.Config) (orbital.TargetOperator, error) {
	opts := amqp.WithNoAuth()
	if cfg.TenantManager.SecretRef.Type == commoncfg.MTLSSecretType {
		opts = operator.WithMTLS(cfg.TenantManager.SecretRef.MTLS)
	}

	amqpClient, err := amqp.NewClient(ctx, codec.Proto{}, amqp.ConnectionInfo{
		URL:    cfg.TenantManager.AMQP.URL,
		Target: cfg.TenantManager.AMQP.Target,
		Source: cfg.TenantManager.AMQP.Source,
	}, opts)
	if err != nil {
		return orbital.TargetOperator{}, oops.In(logDomain).Wrapf(err, "Failed to create AMQP client")
	}

	return orbital.TargetOperator{Client: amqpClient}, nil
}

func validateAndGetClients(cfg *config.Config) (clients.Factory, error) {
	clientsFactory, err := clients.NewFactory(cfg.Services)
	if err != nil {
		return nil, oops.In(logDomain).Wrapf(err, "Failed to create clients factory")
	}

	if clientsFactory.Registry() == nil || !cfg.Services.Registry.Enabled {
		return nil, oops.In(logDomain).Errorf("Registry client is nil, please check gRPC configuration")
	}

	if clientsFactory.SessionManager() == nil || !cfg.Services.SessionManager.Enabled {
		return nil, oops.In(logDomain).Errorf("session-manager client is nil, please check gRPC configuration")
	}

	return clientsFactory, nil
}

func validateConfig(cfg *config.Config) error {
	err := cfg.TenantManager.Validate()
	if err != nil {
		return oops.In(logDomain).Wrapf(err, "failed to validate tenant-manager configuration")
	}

	if cfg.Services.Registry == nil {
		return oops.In(logDomain).Errorf("registry service configuration is required")
	}

	if cfg.Services.SessionManager == nil {
		return oops.In(logDomain).Errorf("session-manager service configuration is required")
	}

	return nil
}
