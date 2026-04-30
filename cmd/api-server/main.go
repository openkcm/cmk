package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/health"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/samber/oops"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/daemon"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/utils/cmd"
	statusserver "github.com/openkcm/cmk/utils/status_server"
)

var (
	BuildInfo               = "{}"
	gracefulShutdownSec     = flag.Int64("graceful-shutdown", 1, "graceful shutdown seconds")
	gracefulShutdownMessage = flag.String("graceful-shutdown-message", "Graceful shutdown in %d seconds",
		"graceful shutdown message")
)

// - Starts the status server
// - Starts the CMK API Server
//
//nolint:funlen
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
	statusserver.StartStatusServer(ctx, cfg, health.WithCheck(health.Check{
		Name: "HTTP Server",
		Check: func(ctx context.Context) error {
			dialer := &net.Dialer{
				Timeout: time.Second * 1,
			}
			conn, err := dialer.DialContext(ctx, "tcp", cfg.HTTP.Address)
			if err != nil {
				return fmt.Errorf("health check: cannot connect to %s: %w", cfg.HTTP.Address, err)
			}
			defer func() { _ = conn.Close() }()

			return nil
		},
	}))

	go daemon.MonitorKeystorePoolSize(ctx, cfg)

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
