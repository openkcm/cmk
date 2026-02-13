package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/health"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/openkcm/common-sdk/pkg/status"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/oops"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/daemon"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/db/dsn"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/manager"
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
	labelKeystore      = "keystore"
)

// - Starts the status server
// - Starts the CMK API Server
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

	log.Debug(ctx, "Starting the application", slog.Any("config", cfg))

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

	// Create and start CMK Server
	s, err := daemon.NewCMKServer(ctx, cfg, dbCon)
	if err != nil {
		return oops.In("main").Wrapf(err, "creating cmk server")
	}

	err = s.Start(ctx)
	if err != nil {
		return oops.In("main").Wrapf(err, "starting cmk api server")
	}

	log.Info(ctx, "API Server has started")

	<-ctx.Done()

	err = s.Close(ctx)
	if err != nil {
		return oops.In("main").Wrapf(err, "closing server")
	}

	return nil
}

func monitorKeystorePoolSize(
	ctx context.Context,
	cfg *config.Config,
) {
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "keystore_pool_available",
			Help: "The number of keystore entries in the pool",
		},
		[]string{
			labelKeystore,
		},
	)

	log.Debug(ctx, "Registering keystore pool size gauge metric")

	dbCon, err := db.StartDBConnection(ctx, cfg.Database, cfg.DatabaseReplicas)
	if err != nil {
		log.Error(ctx, "failed to initialize DB Connection", err)
	}

	pool := manager.NewPool(sql.NewRepository(dbCon))

	ticker := time.NewTicker(cfg.KeystorePool.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info(ctx, "stopping keystore pool size monitoring")
			return
		case <-ticker.C:
			count, err := pool.Count(ctx)
			if err != nil {
				log.Error(ctx, "failed to get keystore pool size", err)
			} else {
				gauge.WithLabelValues(labelKeystore).Set(float64(count))
				log.Debug(ctx, "keystore pool size", slog.Int("size", count))
			}
		}
	}
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
		health.WithCheck(health.Check{
			Name: "HTTP Server",
			Check: func(ctx context.Context) error {
				dialer := &net.Dialer{}

				conn, err := dialer.DialContext(ctx, "tcp", cfg.HTTP.Address)
				if err != nil {
					return fmt.Errorf("health check: cannot connect to %s: %w", cfg.HTTP.Address, err)
				}
				defer func() { _ = conn.Close() }()

				return nil
			},
		}),
	}

	if cfg.Telemetry.Metrics.Prometheus.Enabled {
		go monitorKeystorePoolSize(ctx, cfg)
	}

	go func() {
		err := status.Serve(ctx, &cfg.BaseConfig, healthOptions...)
		if err != nil {
			log.Error(ctx, "Failure on the status server", err)

			_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		}
	}()
}

// main is the entry point for the application. It is intentionally kept small
// because it is hard to test, which would lower test coverage.
func main() {
	flag.Parse()

	exitCode := cmd.RunFuncWithSignalHandling(run, cmd.RunFlags{
		GracefulShutdownSec:     *gracefulShutdownSec,
		GracefulShutdownMessage: *gracefulShutdownMessage,
		Env:                     constants.APIName,
	})
	os.Exit(exitCode)
}
