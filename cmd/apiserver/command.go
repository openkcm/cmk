package apiserver

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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/daemon"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/db/dsn"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/repo/sql"
)

const (
	healthStatusTimeoutS = 5 * time.Second
	postgresDriverName   = "pgx"
	labelKeystore        = "keystore"
)

//nolint:mnd
var defaultConfig = map[string]any{"Certificates": map[string]int{"ValidityDays": 30}}

// - Starts the status server
// - Starts the CMK API Server
func run(ctx context.Context, cfg *config.Config) error {
	// LoggerConfig initialisation
	err := logger.InitAsDefault(cfg.Logger, cfg.Application)
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

	// Create and start CMK Server
	s, err := daemon.NewCMKServer(ctx, cfg)
	if err != nil {
		return oops.In("main").Wrapf(err, "creating cmk server")
	}

	err = s.Start(ctx)
	if err != nil {
		return oops.In("main").Wrapf(err, "starting cmk api server")
	}

	<-ctx.Done()

	err = s.Close(ctx)
	if err != nil {
		return oops.In("main").Wrapf(err, "closing server")
	}

	return nil
}

func monitorKeystorePoolSize(
	ctx context.Context,
	cfg config.Config,
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

	dbCon, err := db.StartDBConnection(cfg.Database, cfg.DatabaseReplicas)
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
	)

	readiness := status.WithReadiness(
		health.NewHandler(
			health.NewChecker(healthOptions...),
		),
	)

	if cfg.Telemetry.Metrics.Prometheus.Enabled {
		go monitorKeystorePoolSize(ctx, *cfg)
	}

	go func() {
		err := status.Start(ctx, &cfg.BaseConfig, liveness, readiness)
		if err != nil {
			log.Error(ctx, "Failure on the status server", err)

			_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		}
	}()
}

func loadConfig(buildInfo string) (*config.Config, error) {
	cfg := &config.Config{}

	loader := commoncfg.NewLoader(
		cfg,
		commoncfg.WithDefaults(defaultConfig),
		commoncfg.WithPaths(
			constants.DefaultConfigPath1,
			constants.DefaultConfigPath2,
			".",
		),
		commoncfg.WithEnvOverride(constants.APIName),
	)

	err := loader.LoadConfig()
	if err != nil {
		return nil, oops.In("main").Wrapf(err, "failed to load config")
	}

	// Update Version
	err = commoncfg.UpdateConfigVersion(&cfg.BaseConfig, buildInfo)
	if err != nil {
		return nil, oops.In("main").
			Wrapf(err, "Failed to update the version configuration")
	}

	err = cfg.Validate()
	if err != nil {
		return nil, oops.In("main").Wrapf(err, "failed to validate config")
	}

	return cfg, nil
}

func Cmd(buildInfo string) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "api-server",
		Short: "CMK API Server",
		Long:  "CMK API Server is a component of the Cloud Key Management system that provides ",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(buildInfo)
			if err != nil {
				return oops.In("main").Wrapf(err, "failed to load config")
			}

			err = run(cmd.Context(), cfg)
			if err != nil {
				return oops.In("main").Wrapf(err, "failed to run the api server")
			}

			return err
		},
	}

	return cmd
}
