package main

import (
	"context"
	"flag"
	"os"
	"syscall"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/health"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/openkcm/common-sdk/pkg/status"
	"github.com/samber/oops"

	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/db/dsn"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/utils/cmd"
)

var (
	BuildInfo               = "{}"
	gracefulShutdownSec     = flag.Int64("graceful-shutdown", 1, "graceful shutdown seconds")
	gracefulShutdownMessage = flag.String("graceful-shutdown-message", "Graceful shutdown in %d seconds",
		"graceful shutdown message")
)

const (
	postgresDriverName = "pgx"
)

// - Starts the status server
// - Starts the event reconciler (orbital manager)
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
	startStatusServer(ctx, cfg)

	// Database initialisation
	dbCon, err := db.StartDB(ctx, cfg)
	if err != nil {
		return oops.In("main").Wrapf(err, "Failed to start database")
	}

	repo := sql.NewRepository(dbCon)

	// GRPC clients factory initialisation
	clientsFactory, err := clients.NewFactory(cfg.Services)
	if err != nil {
		log.Error(ctx, "error connecting to registry service gRPC server", err)
	}

	ctlg, err := catalog.New(ctx, cfg)
	if err != nil {
		log.Error(ctx, "Failed to load plugin", err)
	}

	reconciler, err := eventprocessor.NewCryptoReconciler(ctx, cfg, repo, ctlg, clientsFactory)
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

func startStatusServer(ctx context.Context, cfg *config.Config) {
	dsnFromConfig, err := dsn.FromDBConfig(cfg.Database)
	if err != nil {
		log.Error(ctx, "Could not load DSN from database config", err)
	}

	healthOptions := []health.Option{
		health.WithDatabaseChecker(
			postgresDriverName,
			dsnFromConfig,
		),
	}

	go func() {
		err := status.Serve(ctx, &cfg.BaseConfig, healthOptions...)
		if err != nil {
			log.Error(ctx, "Failure on the status server", err)

			_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		}
	}()
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
