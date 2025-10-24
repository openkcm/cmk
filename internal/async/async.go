package async

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	"github.com/hibiken/asynq"
	"github.com/openkcm/common-sdk/pkg/commoncfg"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk-core/internal/async/tasks"
	conf "github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/db"
	"github.com/openkcm/cmk-core/internal/errs"
	eventprocessor "github.com/openkcm/cmk-core/internal/event-processor"
	"github.com/openkcm/cmk-core/internal/grpc/catalog"
	"github.com/openkcm/cmk-core/internal/log"
	"github.com/openkcm/cmk-core/internal/manager"
	"github.com/openkcm/cmk-core/internal/repo/sql"
)

const (
	// syncInterval is the interval at which the scheduled task manager will check for config changes.
	syncInterval = 10 * time.Second
)

var (
	ErrLoadingDatabaseHost = errors.New("error loading task queue host")
	ErrMTLSRedisClientOpt  = errors.New("error redis client opt")
	ErrSecretTypeQueue     = errors.New("unsupported secret type for task queue")
	ErrACLNotEnabled       = errors.New("ACL is not enabled for task queue")
	ErrACLPassword         = errors.New("ACL is not load password for redis client")
	ErrACLUsername         = errors.New("ACL is not load username for redis client")
)

// TaskHandler defines the interface for handling async
type TaskHandler interface {
	ProcessTask(ctx context.Context, task *asynq.Task) error
	TaskType() string
}

// App manages task processing, scheduling, and worker functionality
type App struct {
	asynqClient        *asynq.Client
	asynqServer        *asynq.Server
	asynqServerCfg     asynq.Config
	taskQueueCfg       asynq.RedisClientOpt
	tasks              map[string]TaskHandler
	systemClient       tasks.SystemUpdater
	certificateClient  tasks.CertUpdater
	hyokClient         tasks.HYOKUpdater
	keystorePoolClient tasks.KeystorePoolUpdater
	cfg                *conf.Config
	dbCon              *multitenancy.DB
}

// New creates a new instance of App
func New(cfg *conf.Config) (*App, error) {
	taskQueueCfg := cfg.Scheduler.TaskQueue

	taskQueueHost, err := commoncfg.LoadValueFromSourceRef(taskQueueCfg.Host)
	if err != nil {
		return nil, errs.Wrap(ErrLoadingDatabaseHost, err)
	}

	var redisOpts asynq.RedisClientOpt

	switch taskQueueCfg.SecretRef.Type {
	case commoncfg.InsecureSecretType:
		taskQueueUsername, taskQueuePassword, err := loadALCAuthFromConfig(taskQueueCfg)
		if err != nil {
			return nil, err
		}

		redisOpts = asynq.RedisClientOpt{
			Addr:     net.JoinHostPort(string(taskQueueHost), taskQueueCfg.Port),
			Password: string(taskQueuePassword),
			Username: string(taskQueueUsername),
		}
	case commoncfg.MTLSSecretType:
		redisOpts, err = buildMTLSRedisClientOpt(taskQueueCfg, taskQueueHost)
		if err != nil {
			return nil, errs.Wrap(ErrMTLSRedisClientOpt, err)
		}
	case commoncfg.ApiTokenSecretType, commoncfg.BasicSecretType, commoncfg.OAuth2SecretType:
		return nil, ErrSecretTypeQueue
	default:
		return nil, ErrSecretTypeQueue
	}

	dbCon, err := db.StartDBConnection(cfg.Database, cfg.DatabaseReplicas)
	if err != nil {
		return nil, errs.Wrap(db.ErrStartingDBCon, err)
	}

	return &App{
		taskQueueCfg: redisOpts,
		asynqClient:  asynq.NewClient(redisOpts),
		tasks:        make(map[string]TaskHandler),
		dbCon:        dbCon,
		cfg:          cfg,
	}, nil
}

// RegisterTasks registers multiple task handlers
func (a *App) RegisterTasks(ctx context.Context, handlers []TaskHandler) {
	for _, handler := range handlers {
		taskType := handler.TaskType()
		a.tasks[taskType] = handler
		log.Info(ctx, "Registered task", slog.String("Name", taskType))
	}
}

// RunWorker starts the worker process to process the tasks
//
//nolint:funlen
func (a *App) RunWorker(ctx context.Context, cfg *conf.Config) error {
	log.Info(ctx, "Starting async worker")

	ctlg, err := catalog.New(ctx, *cfg)
	if err != nil {
		return errs.Wrapf(err, "failed to start loading catalog")
	}

	tenancyRepo := sql.NewRepository(a.dbCon)

	sis, err := manager.NewSystemInformationManager(tenancyRepo, ctlg, &a.cfg.System)
	if err != nil {
		return errs.Wrapf(err, "failed to start system information manager")
	}

	cfg.EventProcessor.Targets = nil // Disable consumer creation in the event processor

	reconciler, err := eventprocessor.NewCryptoReconciler(ctx, cfg, tenancyRepo, ctlg)
	if err != nil {
		return errs.Wrapf(err, "failed to create event reconciler")
	}

	certManager := manager.NewCertificateManager(ctx, tenancyRepo, ctlg, &cfg.Certificates)
	tenantConfigManager := manager.NewTenantConfigManager(tenancyRepo, ctlg)
	keyConfigManager := manager.NewKeyConfigManager(tenancyRepo, certManager, cfg)
	keyManager := manager.NewKeyManager(
		tenancyRepo, ctlg, tenantConfigManager, keyConfigManager, certManager, reconciler, nil)

	a.hyokClient = keyManager
	a.keystorePoolClient = keyManager
	a.systemClient = sis
	a.certificateClient = certManager

	r := sql.NewRepository(a.dbCon)

	log.Info(ctx, "Registering Tasks")
	a.RegisterTasks(ctx,
		[]TaskHandler{
			tasks.NewSystemsRefresher(a.systemClient, r),
			tasks.NewCertRotator(a.certificateClient, r),
			tasks.NewHYOKSync(a.hyokClient, r),
			tasks.NewKeystorePoolFiller(a.keystorePoolClient, r, cfg.KeystorePool),
		})

	a.asynqServer = asynq.NewServer(a.taskQueueCfg, a.asynqServerCfg)

	// Create a new mux and register all task handlers
	mux := asynq.NewServeMux()

	for taskName, handler := range a.tasks {
		h := handler // Create a local copy to avoid closure problems

		mux.HandleFunc(taskName, func(ctx context.Context, task *asynq.Task) error {
			return h.ProcessTask(ctx, task)
		})
	}

	log.Info(ctx, "Starting worker server")

	err = a.asynqServer.Run(mux)
	if err != nil {
		return errs.Wrap(ErrStartingWorker, err)
	}

	return nil
}

// RunScheduler starts the cron job scheduling
// It starts the cron related tasks defined in the schedulerTasksConfig
func (a *App) RunScheduler() error {
	provider := &ScheduledTaskConfigProvider{a.cfg}

	mgr, err := asynq.NewPeriodicTaskManager(
		asynq.PeriodicTaskManagerOpts{
			RedisConnOpt:               a.taskQueueCfg,
			PeriodicTaskConfigProvider: provider,
			SyncInterval:               syncInterval,
		})
	if err != nil {
		return errs.Wrap(ErrCreatingScheduler, err)
	}

	err = mgr.Run()
	if err != nil {
		return errs.Wrap(ErrRunningScheduler, err)
	}

	return nil
}

// EnqueueTask is used to run tasks
func (a *App) EnqueueTask(
	ctx context.Context,
	task *asynq.Task,
	opts ...asynq.Option,
) (*asynq.TaskInfo, error) {
	ctx = log.InjectTask(ctx, task)
	log.Debug(ctx, "Enqueuing task to be processed")

	info, err := a.asynqClient.Enqueue(task, opts...)
	if err != nil {
		return nil, errs.Wrap(ErrEnqueueingTask, err)
	}

	log.Debug(ctx, "Enqueued task")

	return info, nil
}

// Shutdown gracefully shuts down the worker and scheduler
func (a *App) Shutdown(ctx context.Context) error {
	log.Info(ctx, "Starting async app shutdown")

	if a.asynqServer != nil {
		a.asynqServer.Shutdown()
	}

	if a.asynqClient != nil {
		err := a.asynqClient.Close()
		if err != nil {
			return errs.Wrap(ErrClientShutdown, err)
		}
	}

	log.Info(ctx, "Async app shutdown completed")

	return nil
}

func buildMTLSRedisClientOpt(
	taskQueueCfg conf.Redis,
	taskQueueHost []byte,
) (asynq.RedisClientOpt, error) {
	tlsConfig, err := commoncfg.LoadMTLSConfig(&taskQueueCfg.SecretRef.MTLS)
	if err != nil {
		return asynq.RedisClientOpt{}, errs.Wrap(conf.ErrLoadMTLSConfig, err)
	}

	clientOps := asynq.RedisClientOpt{
		Addr:      net.JoinHostPort(string(taskQueueHost), taskQueueCfg.Port),
		TLSConfig: tlsConfig,
	}

	if taskQueueCfg.ACL.Enabled {
		taskQueueUsername, taskQueuePassword, err := loadALCAuthFromConfig(taskQueueCfg)
		if err != nil {
			return asynq.RedisClientOpt{}, err
		}

		clientOps.Username = string(taskQueueUsername)
		clientOps.Password = string(taskQueuePassword)
	}

	return clientOps, nil
}

func loadALCAuthFromConfig(cfg conf.Redis) ([]byte, []byte, error) {
	username, err := commoncfg.LoadValueFromSourceRef(cfg.ACL.Username)
	if err != nil {
		return nil, nil, ErrACLUsername
	}

	password, err := commoncfg.LoadValueFromSourceRef(cfg.ACL.Password)
	if err != nil {
		return nil, nil, ErrACLPassword
	}

	return username, password, nil
}
