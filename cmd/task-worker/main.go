package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/samber/oops"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/async/tasks"
	tenantTask "github.com/openkcm/cmk/internal/async/tasks/tenant"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/errs"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/manager"
	notifierclient "github.com/openkcm/cmk/internal/notifier/client"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	statusserver "github.com/openkcm/cmk/utils/status_server"
)

var (
	BuildInfo               = "{}"
	gracefulShutdownSec     = flag.Int64("graceful-shutdown", 1, "graceful shutdown seconds")
	gracefulShutdownMessage = flag.String("graceful-shutdown-message", "Graceful shutdown in %d seconds",
		"graceful shutdown message")
	ErrRegistryEnabled = errors.New("failed to create registry client")
)

const AppName = "worker"

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
	statusserver.StartStatusServer(ctx, cfg)

	cron, err := async.New(cfg)
	if err != nil {
		return oops.In("main").Wrapf(err, "failed to create the worker")
	}

	dbCon, err := db.StartDBConnection(ctx, cfg.Database, cfg.DatabaseReplicas, &cfg.Telemetry)
	if err != nil {
		return errs.Wrap(db.ErrStartingDBCon, err)
	}
	sqlDB, err := dbCon.DB.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	r := sql.NewRepository(dbCon)

	err = registerTasks(ctx, r, cfg, cron)
	if err != nil {
		return oops.In("main").Wrapf(err, "failed to register tasks")
	}

	err = cron.RunWorker(ctx, r)
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

//nolint:funlen
func registerTasks(
	ctx context.Context,
	r repo.Repo,
	cfg *config.Config,
	cron *async.App,
) error {
	svcRegistry, err := cmkpluginregistry.New(ctx, cfg)
	if err != nil {
		return errs.Wrapf(err, "failed to start loading catalog")
	}

	notifierClient, err := notifierclient.New(ctx, svcRegistry)
	if err != nil {
		return errs.Wrapf(err, "failed to create notification client")
	}

	sis, err := manager.NewSystemInformationManager(r, svcRegistry, &cfg.ContextModels.System)
	if err != nil {
		return errs.Wrapf(err, "failed to start system information manager")
	}

	eventFactory, err := eventprocessor.NewEventFactory(ctx, cfg, r)
	if err != nil {
		return errs.Wrapf(err, "failed to create event factory")
	}

	f, err := clients.NewFactory(cfg.Services)
	if err != nil {
		return err
	}

	if f.Registry() == nil || !cfg.Services.Registry.Enabled {
		return ErrRegistryEnabled
	}

	cmkAuditor := auditor.New(ctx, cfg)
	userManager := manager.NewUserManager(r, cmkAuditor)
	certManager := manager.NewCertificateManager(ctx, r, svcRegistry, cfg)
	tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, cfg)
	tagManager := manager.NewTagManager(r)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, userManager, tagManager, cmkAuditor, cfg)
	keyManager := manager.NewKeyManager(
		r, svcRegistry, tenantConfigManager, keyConfigManager, userManager, certManager, eventFactory, cmkAuditor)
	systemManager := manager.NewSystemManager(ctx, r, nil, eventFactory, svcRegistry, cfg, keyConfigManager, userManager)
	groupManager := manager.NewGroupManager(r, svcRegistry, userManager)
	workflowManager := manager.NewWorkflowManager(r, svcRegistry, keyManager, keyConfigManager, systemManager,
		groupManager, userManager, cron.Client(), tenantConfigManager, cfg)

	taskHandlers := []async.TaskHandler{
		tenantTask.NewSystemsRefresher(sis, r),
		tenantTask.NewCertRotator(certManager, r),
		tasks.NewKeystorePoolFiller(keyManager, r, cfg.KeystorePool),
		tasks.NewWorkflowProcessor(workflowManager, r),
		tasks.NewNotificationSender(notifierClient),
		tenantTask.NewWorkflowExpiryProcessor(workflowManager, r),
		tenantTask.NewWorkflowCleaner(workflowManager, r),
		tenantTask.NewTenantNameRefresher(r, f.Registry()),
		tenantTask.NewHYOKSync(keyManager, r),
	}

	cron.RegisterTasks(ctx, taskHandlers)

	return nil
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
