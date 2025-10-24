package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/health"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/openkcm/common-sdk/pkg/status"
	"github.com/samber/oops"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db/dsn"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/tenant-manager/business"
)

const (
	defaultTimeout            = 5
	errMsgLoadConfig          = "Failed to load the configuration"
	errMsgLoggerInit          = "Failed to initialise the logger"
	errMsgLoadTelemetry       = "Failed to load the telemetry"
	errMsgStartTheBusinessApp = "Failed to start the main business application"
	errMsgStatusServer        = "Failure on the status server"
	errMsgStartApp            = "Failed to start the application"
	postgresDriverName        = "pgx"
)

var (
	gracefulShutdownSec     = flag.Int64("graceful-shutdown", 1, "graceful shutdown seconds")
	gracefulShutdownMessage = flag.String("graceful-shutdown-message", "Graceful shutdown in %d seconds",
		"graceful shutdown message")
)

// run does the heavy lifting until the service is up and running. It will:
//   - Load the config and initializes the logger
//   - Start the status server in a goroutine
//   - Start the business logic and eventually return the error from it
//

func run(ctx context.Context) error {
	// Load Configuration
	cfg, err := loadConfig()
	if err != nil {
		return oops.In("main").
			Wrapf(err, errMsgLoadConfig)
	}

	// LoggerConfig initialisation
	err = logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("main").
			Wrapf(err, errMsgLoggerInit)
	}

	// OpenTelemetry initialisation
	err = otlp.Init(ctx, &cfg.Application, &cfg.Telemetry, &cfg.Logger)
	if err != nil {
		return oops.In("main").
			Wrapf(err, errMsgLoadTelemetry)
	}

	// Status Server Initialisation
	startStatusServer(ctx, cfg)

	// Business Logic
	err = business.Main(ctx, cfg)
	if err != nil {
		return oops.In("main").
			Wrapf(err, errMsgStartTheBusinessApp)
	}

	return nil
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
		health.WithTimeout(defaultTimeout*time.Second),
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
	)

	readiness := status.WithReadiness(
		health.NewHandler(
			health.NewChecker(healthOptions...),
		),
	)

	go func() {
		err := status.Start(ctx, &cfg.BaseConfig, liveness, readiness)
		if err != nil {
			log.Error(ctx, errMsgStatusServer, err)

			_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		}
	}()
}

func loadConfig() (*config.Config, error) {
	cfg := &config.Config{}

	err := commoncfg.LoadConfig(
		cfg,
		map[string]any{},
		"/etc/tenant-manager",
		".",
	)

	return cfg, err
}

// runFuncWithSignalHandling runs the given function with signal handling. When
// a CTRL-C is received, the context will be cancelled on which the function can
// act upon.
func runFuncWithSignalHandling(f func(context.Context) error) int {
	ctx, cancelOnSignal := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGTERM,
	)
	defer cancelOnSignal()

	exitCode := 0

	err := f(ctx)
	if err != nil {
		log.Error(ctx, errMsgStartApp, err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		exitCode = 1
	}

	// graceful shutdown so running goroutines may finish
	_, _ = fmt.Fprintln(os.Stderr, fmt.Sprintf(*gracefulShutdownMessage, *gracefulShutdownSec))
	time.Sleep(time.Duration(*gracefulShutdownSec) * time.Second)

	return exitCode
}

// main is the entry point for the application. It is intentionally kept small
// because it is hard to test, which would lower test coverage.
func main() {
	flag.Parse()

	exitCode := runFuncWithSignalHandling(run)
	os.Exit(exitCode)
}
