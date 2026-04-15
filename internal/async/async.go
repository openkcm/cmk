package async

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	"github.com/hibiken/asynq"
	"github.com/openkcm/common-sdk/pkg/commoncfg"

	conf "github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/repo"
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

type TaskOption func(TenantTaskHandler)

// FanOutHandler task into child tasks per tenant
type FanOutHandler interface {
	TaskHandler

	FanOutFunc() FanOutFunc
}

// TenantTaskHandler is a task that is run for a selection of tenants
type TenantTaskHandler interface {
	TaskHandler
	FanOutHandler

	TenantQuery() *repo.Query
}

// TaskHandler defines the interface for handling async
type TaskHandler interface {
	ProcessTask(ctx context.Context, task *asynq.Task) error
	TaskType() string
}

type Client interface {
	Close() error
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
	Ping() error
}

// App manages task processing, scheduling, and worker functionality

type App struct {
	asynqClient    *asynq.Client
	asynqServer    *asynq.Server
	asynqServerCfg asynq.Config
	taskQueueCfg   asynq.RedisClientOpt
	tasks          map[string]TaskHandler
	middlewares    []Middleware
	cfg            *conf.Config
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

	return &App{
		taskQueueCfg: redisOpts,
		asynqClient:  asynq.NewClient(redisOpts),
		tasks:        make(map[string]TaskHandler),
		middlewares:  getMiddlewares(*cfg),
		cfg:          cfg,
	}, nil
}

// Enqueue is used to run tasks
func (a *App) Enqueue(
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

// RegisterTasks registers multiple task handlers
func (a *App) RegisterTasks(ctx context.Context, handlers []TaskHandler) {
	for _, handler := range handlers {
		a.registerTask(ctx, handler)

		// Check if scheduled task has fanout enabled on config
		taskCfg, ok := a.cfg.Scheduler.GetTasks()[handler.TaskType()]
		if ok && taskCfg.FanOutTask != nil && taskCfg.FanOutTask.Enabled {
			// Configure fanout on tasks that support it
			if h, ok := handler.(FanOutHandler); ok {
				childHandler := NewFanOutHandler(h, h.FanOutFunc())
				a.registerTask(ctx, childHandler)
			}
		}
	}
}

// RunWorker starts the worker process to process the tasks
func (a *App) RunWorker(ctx context.Context, r repo.Repo) error {
	log.Info(ctx, "Starting async worker")

	a.asynqServer = asynq.NewServer(a.taskQueueCfg, a.asynqServerCfg)

	// Create a new mux and register all task handlers
	mux := asynq.NewServeMux()

	for taskName, handler := range a.tasks {
		processTask := handler.ProcessTask
		for _, mw := range a.middlewares {
			processTask = mw(processTask)
		}

		mux.HandleFunc(taskName, func(ctx context.Context, task *asynq.Task) error {
			switch h := handler.(type) {
			case TenantTaskHandler:
				enabled, opts := a.getFanOutOpts(h.TaskType())
				if enabled {
					processor := NewBatchProcessor(
						r,
						WithFanOutTenants(a.Client(), opts...),
						WithTenantQuery(h.TenantQuery()),
					)
					return processor.ProcessTenantsInBatch(ctx, task, h.ProcessTask)
				}
				processor := NewBatchProcessor(r)
				return processor.ProcessTenantsInBatch(ctx, task, h.ProcessTask)
			default:
				return h.ProcessTask(ctx, task)
			}
		})
	}

	log.Info(ctx, "Starting worker server")

	err := a.asynqServer.Run(mux)
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

func (a *App) Client() *asynq.Client {
	return a.asynqClient
}

func (a *App) Inspector() *asynq.Inspector {
	return asynq.NewInspector(a.taskQueueCfg)
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

func getMiddlewares(cfg conf.Config) []Middleware {
	middlewares := []Middleware{
		TracingMiddleware(cfg),
	}

	return middlewares
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

// If FanOutTask is defined and not enabled skip
func (a *App) getFanOutOpts(taskType string) (bool, []asynq.Option) {
	taskCfg, ok := a.cfg.Scheduler.GetTasks()[taskType]
	if !ok || taskCfg.FanOutTask == nil || (taskCfg.FanOutTask != nil && !taskCfg.FanOutTask.Enabled) {
		return false, nil
	}

	opts := []asynq.Option{}
	if taskCfg.FanOutTask.Retries != nil {
		opts = append(opts, asynq.MaxRetry(*taskCfg.FanOutTask.Retries))
	}
	if taskCfg.FanOutTask.TimeOut > 0 {
		opts = append(opts, asynq.Timeout(taskCfg.FanOutTask.TimeOut))
	}

	return true, opts
}

func (a *App) registerTask(ctx context.Context, handler TaskHandler) {
	taskType := handler.TaskType()

	a.tasks[taskType] = handler

	log.Info(
		ctx,
		"Registered task",
		slog.String("name", taskType),
	)
}
