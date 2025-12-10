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

	"github.tools.sap/kms/cmk/internal/async"
	"github.tools.sap/kms/cmk/internal/async/tasks"
	"github.tools.sap/kms/cmk/internal/auditor"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/db"
	"github.tools.sap/kms/cmk/internal/db/dsn"
	"github.tools.sap/kms/cmk/internal/errs"
	eventprocessor "github.tools.sap/kms/cmk/internal/event-processor"
	"github.tools.sap/kms/cmk/internal/grpc/catalog"
	"github.tools.sap/kms/cmk/internal/log"
	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/internal/notifier/client"
	"github.tools.sap/kms/cmk/internal/repo/sql"
)

var (
	BuildInfo               = "{}"
	gracefulShutdownSec     = flag.Int64("graceful-shutdown", 1, "graceful shutdown seconds")
	gracefulShutdownMessage = flag.String("graceful-shutdown-message", "Graceful shutdown in %d seconds",
		"graceful shutdown message")
)

const AppName = "worker"
const (
	healthStatusTimeoutS = 5 * time.Second
	postgresDriverName   = "pgx"
)

// - Starts the status server
// - Starts the Asynq Worker
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

	cron, err := async.New(cfg)
	if err != nil {
		return oops.In("main").Wrapf(err, "failed to create the worker")
	}

	err = registerTasks(ctx, cfg, cron)
	if err != nil {
		return oops.In("main").Wrapf(err, "failed to register tasks")
	}

	err = cron.RunWorker(ctx)
	if err != nil {
		return oops.In("main").Wrapf(err, "failed to start the worker")
	}

	<-ctx.Done()

	err = cron.Shutdown(ctx)
	if err != nil {
		return oops.In("main").Wrapf(err, "%s", async.ErrClientShutdown.Error())
	}

	log.Info(ctx, "shutting down worker")

	return nil
}

func registerTasks(
	ctx context.Context,
	cfg *config.Config,
	cron *async.App,
) error {
	dbCon, err := db.StartDBConnection(ctx, cfg.Database, cfg.DatabaseReplicas)
	if err != nil {
		return errs.Wrap(db.ErrStartingDBCon, err)
	}

	ctlg, err := catalog.New(ctx, cfg)
	if err != nil {
		return errs.Wrapf(err, "failed to start loading catalog")
	}

	r := sql.NewRepository(dbCon)

	sis, err := manager.NewSystemInformationManager(r, ctlg, &cfg.ContextModels.System)
	if err != nil {
		return errs.Wrapf(err, "failed to start system information manager")
	}

	cfg.EventProcessor.Targets = nil // Disable consumer creation in the event processor

	reconciler, err := eventprocessor.NewCryptoReconciler(ctx, cfg, r, ctlg, nil)
	if err != nil {
		return errs.Wrapf(err, "failed to create event reconciler")
	}

	cmkAuditor := auditor.New(ctx, cfg)
	certManager := manager.NewCertificateManager(ctx, r, ctlg, &cfg.Certificates)
	tenantConfigManager := manager.NewTenantConfigManager(r, ctlg)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, cmkAuditor, cfg)
	keyManager := manager.NewKeyManager(
		r, ctlg, tenantConfigManager, keyConfigManager, certManager, reconciler, cmkAuditor)
	systemManager := manager.NewSystemManager(ctx, r, nil, reconciler, ctlg, cfg, keyConfigManager)
	groupManager := manager.NewGroupManager(r, ctlg)
	workflowManager := manager.NewWorkflowManager(r, keyManager, keyConfigManager, systemManager,
		groupManager, cron.Client(), tenantConfigManager)
	notificationClient := client.New(ctx, ctlg)

	cron.RegisterTasks(ctx, []async.TaskHandler{
		tasks.NewSystemsRefresher(sis, r),
		tasks.NewCertRotator(certManager, r),
		tasks.NewHYOKSync(keyManager, r),
		tasks.NewKeystorePoolFiller(keyManager, r, cfg.KeystorePool),
		tasks.NewWorkflowProcessor(workflowManager, r),
		tasks.NewNotificationSender(notificationClient),
		tasks.NewWorkflowCleaner(workflowManager, r),
	})

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

	go func() {
		err := status.Start(ctx, &cfg.BaseConfig, liveness, readiness)
		if err != nil {
			log.Error(ctx, "Failure on the status server", err)

			_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		}
	}()
}

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

	cfg, err := config.LoadConfig(commoncfg.WithEnvOverride(constants.APIName + "_task_worker"))
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

func main() {
	flag.Parse()

	exitCode := runFuncWithSignalHandling(run)
	os.Exit(exitCode)
}
