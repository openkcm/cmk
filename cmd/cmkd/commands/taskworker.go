package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	"github.tools.sap/kms/cmk/internal/async"
	"github.tools.sap/kms/cmk/internal/async/tasks"
	tenantTask "github.tools.sap/kms/cmk/internal/async/tasks/tenant"
	"github.tools.sap/kms/cmk/internal/auditor"
	authz_loader "github.tools.sap/kms/cmk/internal/authz/loader"
	authz_repo "github.tools.sap/kms/cmk/internal/authz/repo"
	"github.tools.sap/kms/cmk/internal/clients"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/db"
	"github.tools.sap/kms/cmk/internal/errs"
	eventprocessor "github.tools.sap/kms/cmk/internal/event-processor"
	"github.tools.sap/kms/cmk/internal/log"
	"github.tools.sap/kms/cmk/internal/manager"
	notifierclient "github.tools.sap/kms/cmk/internal/notifier/client"
	cmkpluginregistry "github.tools.sap/kms/cmk/internal/pluginregistry"
	"github.tools.sap/kms/cmk/internal/repo"
	"github.tools.sap/kms/cmk/internal/repo/sql"
	runcmd "github.tools.sap/kms/cmk/utils/cmd"
	statusserver "github.tools.sap/kms/cmk/utils/status_server"
)

var (
	ErrAuthzLoader     = errors.New("failed to create authz loader")
	ErrRegistryEnabled = errors.New("failed to create registry client")
)

func NewTaskWorker() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task-worker",
		Short: "Start the task worker",
		Long:  `Starts the async task worker that processes background jobs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			exitCode := runcmd.RunFuncWithSignalHandling(runTaskWorker, runcmd.RunFlags{
				GracefulShutdownSec:     gracefulShutdownSec,
				GracefulShutdownMessage: gracefulShutdownMessage,
				Env:                     constants.APIName + "_task_worker",
			})
			if exitCode != 0 {
				return fmt.Errorf("%w: task-worker exited with code %d", ErrNonZeroExit, exitCode)
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&gracefulShutdownSec, "graceful-shutdown", 1, "graceful shutdown seconds")
	cmd.Flags().StringVar(&gracefulShutdownMessage, "graceful-shutdown-message",
		"Graceful shutdown in %d seconds", "graceful shutdown message")

	return cmd
}

func runTaskWorker(ctx context.Context, cfg *config.Config) error {
	err := commoncfg.UpdateConfigVersion(&cfg.BaseConfig, BuildInfo)
	if err != nil {
		return oops.In("task-worker").Wrapf(err, "Failed to update the version configuration")
	}

	err = logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("task-worker").Wrapf(err, "Failed to initialise the logger")
	}

	err = otlp.Init(ctx, &cfg.Application, &cfg.Telemetry, &cfg.Logger)
	if err != nil {
		return oops.In("task-worker").Wrapf(err, "Failed to load the telemetry")
	}

	statusserver.StartStatusServer(ctx, cfg)

	cron, err := async.New(cfg)
	if err != nil {
		return oops.In("task-worker").Wrapf(err, "failed to create the worker")
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
		return oops.In("task-worker").Wrapf(err, "failed to register tasks")
	}

	err = cron.RunWorker(ctx, r)
	if err != nil {
		return oops.In("task-worker").Wrapf(err, "failed to start the worker")
	}

	<-ctx.Done()

	err = cron.Shutdown(ctx)
	if err != nil {
		return oops.In("task-worker").Wrapf(err, "%s", async.ErrClientShutdown.Error())
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
	keyConfigManager := manager.NewKeyConfigManager(
		authzRepo,
		certManager,
		userManager,
		tagManager,
		cmkAuditor,
		eventFactory,
		cfg,
	)
	keyManager := manager.NewKeyManager(
		authzRepo,
		svcRegistry,
		tenantConfigManager,
		keyConfigManager,
		userManager,
		certManager,
		eventFactory,
		cmkAuditor,
	)
	systemManager := manager.NewSystemManager(
		ctx,
		authzRepo,
		authzRepoLoader,
		nil,
		eventFactory,
		svcRegistry,
		cfg,
		keyConfigManager,
		userManager,
	)
	groupManager := manager.NewGroupManager(authzRepo, svcRegistry, userManager)
	workflowManager := manager.NewWorkflowManager(
		authzRepo,
		svcRegistry,
		keyManager,
		keyConfigManager,
		systemManager,
		groupManager,
		userManager,
		cron.Client(),
		tenantConfigManager,
		cfg,
	)

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
