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
	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/openkcm/orbital/codec"
	"github.com/samber/oops"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	oidcmappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"

	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/db/dsn"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/operator"
)

const (
	defaultTimeout        = 5
	errMsgLoadConfig      = "Failed to load the configuration"
	errMsgLoggerInit      = "Failed to initialise the logger"
	errMsgLoadTelemetry   = "Failed to load the telemetry"
	errMsgStatusServer    = "Failure on the status server"
	errMsgStartApp        = "Failed to start the application"
	errMsgRunningOperator = "Failed to run operator"
	postgresDriverName    = "pgx"
	logDomain             = "main"
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

func run(ctx context.Context, cfg *config.Config) error {
	// Validate configuration
	err := validateConfig(cfg)
	if err != nil {
		return err
	}

	err = initializeLoggerAndTelemetry(ctx, cfg)
	if err != nil {
		return err
	}

	startStatusServer(ctx, cfg)

	dbConn, err := db.StartDB(ctx, cfg.Database, cfg.Provisioning, nil)
	if err != nil {
		return oops.In(logDomain).Wrapf(err, "Failed to start the database connection")
	}

	target, err := createAMQPClient(ctx, cfg)
	if err != nil {
		return err
	}

	tenantClient, sessionManagerClient, err := validateAndGetClients(cfg)
	if err != nil {
		return err
	}

	ctlg, err := catalog.New(ctx, *cfg)
	if err != nil {
		return err
	}

	tenantOperator, err := operator.NewTenantOperator(dbConn, target, tenantClient, sessionManagerClient, ctlg)
	if err != nil {
		return oops.In(logDomain).
			Wrapf(err, errMsgRunningOperator)
	}

	return tenantOperator.RunOperator(ctx)
}

func initializeLoggerAndTelemetry(ctx context.Context, cfg *config.Config) error {
	err := logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In(logDomain).
			Wrapf(err, errMsgLoggerInit)
	}

	err = otlp.Init(ctx, &cfg.Application, &cfg.Telemetry, &cfg.Logger)
	if err != nil {
		return oops.In(logDomain).
			Wrapf(err, errMsgLoadTelemetry)
	}

	return nil
}

func createAMQPClient(ctx context.Context, cfg *config.Config) (orbital.OperatorTarget, error) {
	opts := amqp.WithNoAuth()
	if cfg.TenantManager.SecretRef.Type == commoncfg.MTLSSecretType {
		opts = operator.WithMTLS(cfg.TenantManager.SecretRef.MTLS)
	}

	target, err := createOperatorTarget(ctx, cfg, opts)
	if err != nil {
		return orbital.OperatorTarget{}, oops.In(logDomain).
			Wrapf(err, "Failed to create AMQP client")
	}

	return target, nil
}

func createOperatorTarget(ctx context.Context, cfg *config.Config, opts amqp.ClientOption) (orbital.OperatorTarget, error) { //nolint:lll
	amqpClient, err := amqp.NewClient(ctx, codec.Proto{}, amqp.ConnectionInfo{
		URL:    cfg.TenantManager.AMQP.URL,
		Target: cfg.TenantManager.AMQP.Target,
		Source: cfg.TenantManager.AMQP.Source,
	}, opts)
	if err != nil {
		return orbital.OperatorTarget{}, oops.In(logDomain).
			Wrapf(err, "Failed to create AMQP client: %v", err)
	}

	return orbital.OperatorTarget{Client: amqpClient}, nil
}

func validateAndGetClients(cfg *config.Config) (
	tenantgrpc.ServiceClient,
	oidcmappinggrpc.ServiceClient,
	error,
) {
	clientsFactory, err := clients.NewFactory(cfg.Services)
	if err != nil {
		return nil, nil, oops.In(logDomain).
			Wrapf(err, "Failed to create clients factory")
	}

	if clientsFactory.RegistryService() == nil {
		return nil, nil, oops.In(logDomain).
			Errorf("Registry client is nil, please check gRPC configuration")
	}

	if clientsFactory.SessionManager() == nil {
		return nil, nil, oops.In(logDomain).
			Errorf("session-manager client is nil, please check gRPC configuration")
	}

	tenantClient := clientsFactory.RegistryService().Tenant()
	sessionManagerClient := clientsFactory.SessionManager().OIDCMapping()

	return tenantClient, sessionManagerClient, nil
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
	if err != nil {
		return nil, err
	}

	return cfg, err
}

// runFuncWithSignalHandling runs the given function with signal handling. When
// a CTRL-C is received, the context will be cancelled on which the function can
// act upon.
func runFuncWithSignalHandling(f func(context.Context, *config.Config) error) int {
	ctx, cancelOnSignal := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGTERM,
	)
	defer cancelOnSignal()

	exitCode := 0

	// Load Configuration
	cfg, err := loadConfig()
	if err != nil {
		log.Error(ctx, errMsgLoadConfig, err)
		_, _ = fmt.Fprintln(os.Stderr, err)

		exitCode = 1
	}

	err = f(ctx, cfg)
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

// validateConfig

// validateConfig validates the configuration before starting services
func validateConfig(cfg *config.Config) error {
	err := cfg.TenantManager.Validate()
	if err != nil {
		return oops.In(logDomain).
			Wrapf(err, "failed to validate tenant-manager configuration")
	}

	if cfg.Services.Registry == nil {
		return oops.In(logDomain).
			Errorf("registry service configuration is required")
	}

	if cfg.Services.SessionManager == nil {
		return oops.In(logDomain).
			Errorf("session-manager service configuration is required")
	}

	return nil
}
