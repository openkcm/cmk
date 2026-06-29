package commands

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/health"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/daemon"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/log"
	runcmd "github.com/openkcm/cmk/utils/cmd"
	statusserver "github.com/openkcm/cmk/utils/status_server"
)

func NewAPIServer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api-server",
		Short: "Start the CMK API server",
		Long:  `Starts the CMK API server that handles HTTP REST API requests.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			exitCode := runcmd.RunFuncWithSignalHandling(runAPIServer, runcmd.RunFlags{
				GracefulShutdownSec:     gracefulShutdownSec,
				GracefulShutdownMessage: gracefulShutdownMessage,
				Env:                     constants.APIName,
			})
			if exitCode != 0 {
				return fmt.Errorf("%w: api-server exited with code %d", ErrNonZeroExit, exitCode)
			}
			return nil
		},
	}

	cmd.Flags().Int64Var(&gracefulShutdownSec, "graceful-shutdown", 1, "graceful shutdown seconds")
	cmd.Flags().StringVar(&gracefulShutdownMessage, "graceful-shutdown-message",
		"Graceful shutdown in %d seconds", "graceful shutdown message")

	return cmd
}

func runAPIServer(ctx context.Context, cfg *config.Config) error {
	err := commoncfg.UpdateConfigVersion(&cfg.BaseConfig, BuildInfo)
	if err != nil {
		return oops.In("api-server").Wrapf(err, "Failed to update the version configuration")
	}

	err = logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("api-server").Wrapf(err, "Failed to initialise the logger")
	}

	log.Debug(ctx, "Starting the application", slog.Any("config", cfg))

	err = otlp.Init(ctx, &cfg.Application, &cfg.Telemetry, &cfg.Logger)
	if err != nil {
		return oops.In("api-server").Wrapf(err, "Failed to load the telemetry")
	}

	statusserver.StartStatusServer(ctx, cfg, health.WithCheck(health.Check{
		Name: "HTTP Server",
		Check: func(ctx context.Context) error {
			dialer := &net.Dialer{Timeout: time.Second * 1}
			conn, err := dialer.DialContext(ctx, "tcp", cfg.HTTP.Address)
			if err != nil {
				return fmt.Errorf("health check: cannot connect to %s: %w", cfg.HTTP.Address, err)
			}
			defer func() { _ = conn.Close() }()
			return nil
		},
	}))

	go daemon.MonitorKeystorePoolSize(ctx, cfg)

	dbCon, err := db.StartDB(ctx, cfg)
	if err != nil {
		return oops.In("api-server").Wrapf(err, "Failed to start database")
	}

	s, err := daemon.NewCMKServer(ctx, cfg, dbCon)
	if err != nil {
		return oops.In("api-server").Wrapf(err, "creating cmk server")
	}

	err = s.Start(ctx)
	if err != nil {
		return oops.In("api-server").Wrapf(err, "starting cmk api server")
	}

	log.Info(ctx, "API Server has started")

	<-ctx.Done()

	err = s.Close(ctx)
	if err != nil {
		return oops.In("api-server").Wrapf(err, "closing server")
	}

	return nil
}
