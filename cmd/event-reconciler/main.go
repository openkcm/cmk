package main

import (
	"context"
	"flag"
	"os"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/samber/oops"

	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/db"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/log"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/utils/cmd"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	statusserver "github.com/openkcm/cmk/utils/status_server"
)

var (
	BuildInfo               = "{}"
	gracefulShutdownSec     = flag.Int64("graceful-shutdown", 1, "graceful shutdown seconds")
	gracefulShutdownMessage = flag.String("graceful-shutdown-message", "Graceful shutdown in %d seconds",
		"graceful shutdown message")
)

// - Starts the status server
// - Starts the event reconciler (orbital manager)
//
//nolint:cyclop,funlen
func run(ctx context.Context, cfg *config.Config) error {
	// Update Version
	err := commoncfg.UpdateConfigVersion(&cfg.BaseConfig, BuildInfo)
	if err != nil {
		return oops.In("main").
			Wrapf(err, "Failed to update the version configuration")
	}

	// LoggerConfig initialisation
	err = logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("main").
			Wrapf(err, "Failed to initialise the logger")
	}

	// OpenTelemetry initialisation
	err = otlp.Init(ctx, &cfg.Application, &cfg.Telemetry, &cfg.Logger)
	if err != nil {
		return oops.In("main").
			Wrapf(err, "Failed to load the telemetry")
	}

	// Start status server
	statusserver.StartStatusServer(ctx, cfg)

	// Database initialisation
	dbCon, err := db.StartDB(ctx, cfg)
	if err != nil {
		return oops.In("main").Wrapf(err, "Failed to start database")
	}

	repo := sql.NewRepository(dbCon)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(ctx, repo, cfg)
	if authzRepoLoader.AuthzHandler == nil {
		return oops.In("main").Wrapf(err, "Failed to initialise authz loader")
	}

	authzRepo := authz_repo.NewAuthzRepo(repo, authzRepoLoader)

	// GRPC clients factory initialisation
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
		return oops.In("main").Wrapf(err, "Failed injecting authz role")
	}

	reconciler, err := eventprocessor.NewCryptoReconciler(ctx, cfg, authzRepo, svcRegistry, clientsFactory)
	if err != nil {
		return oops.In("main").Wrapf(err, "Failed to create crypto reconciler")
	}

	err = reconciler.Start(ctx)
	if err != nil {
		return oops.In("main").Wrapf(err, "Failed to start crypto reconciler")
	}

	log.Info(ctx, "event reconciler started successfully")

	<-ctx.Done()
	reconciler.CloseAmqpClients(ctx)

	return nil
}

func main() {
	flag.Parse()

	exitCode := cmd.RunFuncWithSignalHandling(run, cmd.RunFlags{
		GracefulShutdownSec:     *gracefulShutdownSec,
		GracefulShutdownMessage: *gracefulShutdownMessage,
		Env:                     "event_reconciler",
	})
	os.Exit(exitCode)
}
