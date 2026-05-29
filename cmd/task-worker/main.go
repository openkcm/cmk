package main

import (
	"context"
	"errors"
	"flag"
	"os"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/samber/oops"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/async/tasks"
	tenantTask "github.com/openkcm/cmk/internal/async/tasks/tenant"
	"github.com/openkcm/cmk/internal/auditor"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
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
	"github.com/openkcm/cmk/utils/cmd"
	statusserver "github.com/openkcm/cmk/utils/status_server"
)

var (
	BuildInfo               = "{}"
	gracefulShutdownSec     = flag.Int64("graceful-shutdown", 1, "graceful shutdown seconds")
	gracefulShutdownMessage = flag.String("graceful-shutdown-message", "Graceful shutdown in %d seconds",
		"graceful shutdown message")
	ErrAuthzLoader     = errors.New("failed to create authz loader")
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
	authzRepoLoader := authz_loader.NewRepoAuthzLoader(ctx, r, cfg)
	if authzRepoLoader.AuthzHandler == nil {
		return ErrAuthzLoader
	}

	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	svcRegistry, err := cmkpluginregistry.New(ctx, cfg)
	if err != nil {
		return errs.Wrapf(err, "failed to start loading catalog")
	}

	notifierClient, err := notifierclient.New(ctx, svcRegistry)
	if err != nil {
		return errs.Wrapf(err, "failed to create notification client")
	}

	sis, err := manager.NewSystemInformationManager(authzRepo, authzRepoLoader, svcRegistry, &cfg.ContextModels.System)
	if err != nil {
		return errs.Wrapf(err, "failed to start system information manager")
	}

	eventFactory, err := eventprocessor.NewEventFactory(ctx, cfg, authzRepo)
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
	userManager := manager.NewUserManager(authzRepo, cmkAuditor)
	certManager := manager.NewCertificateManager(ctx, authzRepo, svcRegistry, cfg)
	tenantConfigManager := manager.NewTenantConfigManager(authzRepo, svcRegistry, cfg)
	tagManager := manager.NewTagManager(authzRepo)
	keyConfigManager := manager.NewKeyConfigManager(authzRepo, certManager, userManager, tagManager, cmkAuditor, cfg)
	keyManager := manager.NewKeyManager(
		authzRepo, svcRegistry, tenantConfigManager, keyConfigManager, userManager, certManager, eventFactory, cmkAuditor)
	systemManager := manager.NewSystemManager(ctx, authzRepo, authzRepoLoader, nil, eventFactory,
		svcRegistry, cfg, keyConfigManager, userManager)
	groupManager := manager.NewGroupManager(authzRepo, svcRegistry, userManager)
	workflowManager := manager.NewWorkflowManager(authzRepo, svcRegistry, keyManager, keyConfigManager, systemManager,
		groupManager, userManager, cron.Client(), tenantConfigManager, cfg)

	taskHandlers := []async.TaskHandler{
		tenantTask.NewSystemsRefresher(sis, authzRepo),
		tenantTask.NewCertRotator(certManager, authzRepo),
		tasks.NewKeystorePoolFiller(keyManager, authzRepo, cfg.KeystorePool),
		tasks.NewWorkflowProcessor(workflowManager, authzRepo),
		tasks.NewNotificationSender(notifierClient),
		tenantTask.NewWorkflowExpiryProcessor(workflowManager, authzRepo),
		tenantTask.NewWorkflowCleaner(workflowManager, authzRepo),
		tenantTask.NewTenantNameRefresher(authzRepo, f.Registry()),
		tenantTask.NewHYOKSync(keyManager, authzRepo),
	}

	cron.RegisterTasks(ctx, taskHandlers)

	return nil
}

func main() {
	flag.Parse()

	exitCode := cmd.RunFuncWithSignalHandling(run, cmd.RunFlags{
		GracefulShutdownSec:     *gracefulShutdownSec,
		GracefulShutdownMessage: *gracefulShutdownMessage,
		Env:                     constants.APIName + "_task_worker",
	})
	os.Exit(exitCode)
}
