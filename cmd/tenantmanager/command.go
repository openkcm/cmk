package tenantmanager

import (
	"context"
	"log/slog"
	"syscall"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/health"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/openkcm/common-sdk/pkg/status"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/cmd/tenantmanager/cli"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/db/dsn"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/tenant-manager/business"
)

const (
	defaultTimeout            = 5
	errMsgLoadConfig          = "Failed to load the configuration"
	errMsgLoggerInit          = "Failed to initialise the logger"
	errMsgLoadTelemetry       = "Failed to load the telemetry"
	errMsgStartTheBusinessApp = "Failed to start the main business application"
	errMsgStatusServer        = "Failure on the status server"
	postgresDriverName        = "pgx"
)

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
		constants.DefaultConfigPath1,
		constants.DefaultConfigPath2,
		".",
	)

	return cfg, err
}

func Cmd(buildInfo string) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "tenant-manager",
		Short: "CMK the Tenant Manager",
		Long:  `CMK Tenant Manager - a service to manage tenants.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			// Load Configuration
			cfg, err := loadConfig()
			if err != nil {
				return oops.In("main").
					Wrapf(err, errMsgLoadConfig)
			}

			// Update Version
			err = commoncfg.UpdateConfigVersion(&cfg.BaseConfig, buildInfo)
			if err != nil {
				return oops.In("main").
					Wrapf(err, "Failed to update the version configuration")
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
		},
	}

	cmd.AddCommand(
		cli.Cmd(),
	)

	return cmd
}
