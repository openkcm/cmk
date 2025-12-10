package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/health"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/openkcm/common-sdk/pkg/status"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/oops"

	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/daemon"
	"github.tools.sap/kms/cmk/internal/db"
	"github.tools.sap/kms/cmk/internal/db/dsn"
	"github.tools.sap/kms/cmk/internal/log"
	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/internal/repo/sql"
)

var (
	BuildInfo               = "{}"
	gracefulShutdownSec     = flag.Int64("graceful-shutdown", 1, "graceful shutdown seconds")
	gracefulShutdownMessage = flag.String("graceful-shutdown-message", "Graceful shutdown in %d seconds",
		"graceful shutdown message")
)

const (
	healthStatusTimeoutS = 5 * time.Second
	postgresDriverName   = "pgx"
	labelKeystore        = "keystore"
)

// runFuncWithSignalHandling runs the given function with signal handling. When
// a CTRL-C is received, the context will be cancelled on which the function can
// act upon.
// It returns the exitCode
func runFuncWithSignalHandling(f func(context.Context, *config.Config) error) int {
	ctx, cancelOnSignal := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGTERM,
	)
	defer cancelOnSignal()

	cfg, err := config.LoadConfig(
		commoncfg.WithEnvOverride(constants.APIName),
	)
	if err != nil {
		log.Error(ctx, "Failed to load the configuration", err)
		_, _ = fmt.Fprintln(os.Stderr, err)

		return 1
	}

	log.Debug(ctx, "Starting the application", slog.Any("config", *cfg))

	err = f(ctx, cfg)
	if err != nil {
		log.Error(ctx, "Failed to start the application", err)
		_, _ = fmt.Fprintln(os.Stderr, err)

		return 1
	}

	// graceful shutdown so running goroutines may finish
	_, _ = fmt.Fprintln(os.Stderr, fmt.Sprintf(*gracefulShutdownMessage, *gracefulShutdownSec))
	time.Sleep(time.Duration(*gracefulShutdownSec) * time.Second)

	return 0
}

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
	liveness := status.WithLiveness(
		health.NewHandler(
			health.NewChecker(health.WithDisabledAutostart()),
		),
	)

	healthOptions := make([]health.Option, 0)
	healthOptions = append(healthOptions,
		health.WithDisabledAutostart(),
		health.WithTimeout(healthStatusTimeoutS),
		health.WithStatusListener(func(ctx context.Context, state health.State) {
			log.Info(ctx, "readiness status changed", slog.String("status", string(state.Status)))
		}),
	)

	dsnFromConfig, err := dsn.FromDBConfig(cfg.Database)
	if err != nil {
		log.Error(ctx, "Could not load DSN from database config", err)
	}

	healthOptions = append(healthOptions,
		health.WithDatabaseChecker(
			postgresDriverName,
			dsnFromConfig,
		),
		health.WithCheck(health.Check{
			Name: "HTTP Server",
			Check: func(ctx context.Context) error {
				conn, err := net.DialTimeout("tcp", cfg.HTTP.Address, 1*time.Second)
				if err != nil {
					return fmt.Errorf("%s health check failed on connect: %w", cfg.HTTP.Address, err)
				}
				conn.Close()
				return nil
			},
		}),
	)

	readiness := status.WithReadiness(
		health.NewHandler(
			health.NewChecker(healthOptions...),
		),
	)

	if cfg.Telemetry.Metrics.Prometheus.Enabled {
		go monitorKeystorePoolSize(ctx, cfg)
	}

	go func() {
		err := status.Start(ctx, &cfg.BaseConfig, liveness, readiness)
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

	exitCode := runFuncWithSignalHandling(run)
	os.Exit(exitCode)
}
