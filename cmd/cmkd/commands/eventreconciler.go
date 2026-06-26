package commands

import (
	"context"
	"fmt"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	authz_loader "github.tools.sap/kms/cmk/internal/authz/loader"
	authz_repo "github.tools.sap/kms/cmk/internal/authz/repo"
	"github.tools.sap/kms/cmk/internal/clients"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/db"
	eventprocessor "github.tools.sap/kms/cmk/internal/event-processor"
	"github.tools.sap/kms/cmk/internal/log"
	cmkpluginregistry "github.tools.sap/kms/cmk/internal/pluginregistry"
	"github.tools.sap/kms/cmk/internal/repo/sql"
	runcmd "github.tools.sap/kms/cmk/utils/cmd"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
	statusserver "github.tools.sap/kms/cmk/utils/status_server"
)

func NewEventReconciler() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event-reconciler",
		Short: "Start the CMK event reconciler",
		Long:  `Starts the CMK event reconciler that processes events from AMQP message brokers via Orbital.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			exitCode := runcmd.RunFuncWithSignalHandling(runEventReconciler, runcmd.RunFlags{
				GracefulShutdownSec:     gracefulShutdownSec,
				GracefulShutdownMessage: gracefulShutdownMessage,
				Env:                     "event_reconciler",
			})
			if exitCode != 0 {
				return fmt.Errorf("event-reconciler exited with code %d", exitCode)
			}
			return nil
		},
	}

	cmd.Flags().Int64Var(&gracefulShutdownSec, "graceful-shutdown", 1, "graceful shutdown seconds")
	cmd.Flags().StringVar(&gracefulShutdownMessage, "graceful-shutdown-message",
		"Graceful shutdown in %d seconds", "graceful shutdown message")

	return cmd
}

//nolint:cyclop
func runEventReconciler(ctx context.Context, cfg *config.Config) error {
	err := commoncfg.UpdateConfigVersion(&cfg.BaseConfig, BuildInfo)
	if err != nil {
		return oops.In("event-reconciler").Wrapf(err, "Failed to update the version configuration")
	}

	err = logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("event-reconciler").Wrapf(err, "Failed to initialise the logger")
	}

	err = otlp.Init(ctx, &cfg.Application, &cfg.Telemetry, &cfg.Logger)
	if err != nil {
		return oops.In("event-reconciler").Wrapf(err, "Failed to load the telemetry")
	}

	statusserver.StartStatusServer(ctx, cfg)

	dbCon, err := db.StartDB(ctx, cfg)
	if err != nil {
		return oops.In("event-reconciler").Wrapf(err, "Failed to start database")
	}

	repo := sql.NewRepository(dbCon)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(ctx, repo, cfg)
	if authzRepoLoader.AuthzHandler == nil {
		return oops.In("event-reconciler").Wrapf(err, "Failed to initialise authz loader")
	}

	authzRepo := authz_repo.NewAuthzRepo(repo, authzRepoLoader)

	clientsFactory, err := clients.NewFactory(cfg.Services)
	if err != nil {
		log.Error(ctx, "error connecting to registry service gRPC server", err)
	}

	svcRegistry, err := cmkpluginregistry.New(ctx, cfg)
	if err != nil {
		log.Error(ctx, "Failed to load plugin", err)
	}

	ctx, err = cmkcontext.InjectInternalUserData(ctx, constants.InternalEventReconcilerRole)
	if err != nil {
		return oops.In("event-reconciler").Wrapf(err, "Failed injecting authz role")
	}

	reconciler, err := eventprocessor.NewCryptoReconciler(ctx, cfg, authzRepo, svcRegistry, clientsFactory)
	if err != nil {
		return oops.In("event-reconciler").Wrapf(err, "Failed to create crypto reconciler")
	}

	err = reconciler.Start(ctx)
	if err != nil {
		return oops.In("event-reconciler").Wrapf(err, "Failed to start crypto reconciler")
	}

	log.Info(ctx, "Event Reconciler has started")

	<-ctx.Done()
	reconciler.CloseAmqpClients(ctx)

	return nil
}
