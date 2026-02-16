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

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/async/tasks"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/db/dsn"
	"github.com/openkcm/cmk/internal/errs"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/notifier/client"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo/sql"
)

var (
	BuildInfo               = "{}"
	gracefulShutdownSec     = flag.Int64("graceful-shutdown", 1, "graceful shutdown seconds")
	gracefulShutdownMessage = flag.String("graceful-shutdown-message", "Graceful shutdown in %d seconds",
		"graceful shutdown message")
)

const AppName = "worker"
const (
	postgresDriverName = "pgx"
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

	svcRegistry, err := cmkpluginregistry.New(ctx, cfg)
	if err != nil {
		return errs.Wrapf(err, "failed to start loading catalog")
	}

	r := sql.NewRepository(dbCon)

	sis, err := manager.NewSystemInformationManager(r, svcRegistry, &cfg.ContextModels.System)
	if err != nil {
		return errs.Wrapf(err, "failed to start system information manager")
	}

	cfg.EventProcessor.Targets = nil // Disable consumer creation in the event processor

	reconciler, err := eventprocessor.NewCryptoReconciler(ctx, cfg, r, svcRegistry, nil)
	if err != nil {
		return errs.Wrapf(err, "failed to create event reconciler")
	}

	cmkAuditor := auditor.New(ctx, cfg)
	userManager := manager.NewUserManager(r, cmkAuditor)
	certManager := manager.NewCertificateManager(ctx, r, svcRegistry, &cfg.Certificates)
	tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, cfg)
	tagManager := manager.NewTagManager(r)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, userManager, tagManager, cmkAuditor, cfg)
	keyManager := manager.NewKeyManager(
		r, svcRegistry, tenantConfigManager, keyConfigManager, userManager, certManager, reconciler, cmkAuditor)
	systemManager := manager.NewSystemManager(ctx, r, nil, reconciler, svcRegistry, cfg, keyConfigManager, userManager)
	groupManager := manager.NewGroupManager(r, svcRegistry, userManager)
	workflowManager := manager.NewWorkflowManager(r, keyManager, keyConfigManager, systemManager,
		groupManager, userManager, cron.Client(), tenantConfigManager, cfg)
	notificationClient := client.New(ctx, svcRegistry)

	cron.RegisterTasks(ctx, []async.TaskHandler{
		tasks.NewSystemsRefresher(sis, r),
		tasks.NewCertRotator(certManager, r),
		tasks.NewHYOKSync(keyManager, r),
		tasks.NewKeystorePoolFiller(keyManager, r, cfg.KeystorePool),
		tasks.NewWorkflowProcessor(workflowManager, r),
		tasks.NewNotificationSender(notificationClient),
		tasks.NewWorkflowExpiryProcessor(workflowManager, r),
		tasks.NewWorkflowCleaner(workflowManager, r),
	})

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
