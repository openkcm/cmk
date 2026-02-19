package eventprocessor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/openkcm/orbital/codec"

	_ "github.com/lib/pq" // Import PostgreSQL driver to initialize the database connection

	goAmqp "github.com/Azure/go-amqp"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/clients/registry"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo"
)

const (
	// defaultMaxReconcileCount If want to limit the reconcile period for one task to one day,
	// need maxReconcileCount = 18, as there is an exponential backoff for retries,
	// starting with 10s and limiting at 10240s.
	defaultMaxReconcileCount = 18

	EmbeddedTarget = "embedded"
)

var (
	ErrTargetNotConfigured       = errors.New("target not configured for region")
	ErrUnsupportedTaskType       = errors.New("unsupported task type")
	ErrKeyAccessMetadataNotFound = errors.New("key access metadata not found for system region")
	ErrPluginNotFound            = errors.New("plugin not found for key provider")
	ErrSettingKeyClaim           = errors.New("error setting key claim for system")
	ErrUnsupportedRegion         = errors.New("unsupported region")
	ErrNoConnectedRegionsForKey  = errors.New("no connected regions found for key")
)

type Option func(manager *orbital.Manager)

func WithMaxReconcileCount(n uint64) Option {
	return func(m *orbital.Manager) {
		m.Config.MaxReconcileCount = n
	}
}

func WithConfirmJobAfter(d time.Duration) Option {
	return func(m *orbital.Manager) {
		m.Config.ConfirmJobAfter = d
	}
}

func WithExecInterval(d time.Duration) Option {
	return func(m *orbital.Manager) {
		m.Config.ReconcileWorkerConfig.ExecInterval = d
		m.Config.CreateTasksWorkerConfig.ExecInterval = d
		m.Config.ConfirmJobWorkerConfig.ExecInterval = d
		m.Config.NotifyWorkerConfig.ExecInterval = d
	}
}

// CryptoReconciler is responsible for handling orbital jobs and managing the lifecycle of systems in CMK.
type CryptoReconciler struct {
	repo        repo.Repo
	manager     *orbital.Manager
	targets     map[string]struct{}
	initiators  []orbital.Initiator
	svcRegistry *cmkpluginregistry.Registry
	cmkAuditor  *auditor.Auditor
	registry    registry.Service

	jobHandlerMap map[JobType]JobHandler
}

// NewCryptoReconciler creates a new CryptoReconciler instance.
//
//nolint:funlen
func NewCryptoReconciler(
	ctx context.Context,
	cfg *config.Config,
	repository repo.Repo,
	svcRegistry *cmkpluginregistry.Registry,
	clientsFactory clients.Factory,
	opts ...Option,
) (*CryptoReconciler, error) {
	orbRepo, err := createOrbitalRepository(ctx, cfg.Database)
	if err != nil {
		return nil, errs.Wrapf(err, "failed to create orbital repository")
	}

	targets, err := createAMQPTargets(ctx, &cfg.EventProcessor)
	if err != nil {
		return nil, errs.Wrapf(err, "failed to create targets")
	}

	targetMap := make(map[string]struct{})
	initiators := make([]orbital.Initiator, 0)

	for region := range targets {
		targetMap[region] = struct{}{}
		initiators = append(initiators, targets[region].Client)
	}

	cmkAuditor := auditor.New(ctx, cfg)

	reconciler := &CryptoReconciler{
		repo:        repository,
		targets:     targetMap,
		initiators:  initiators,
		svcRegistry: svcRegistry,
		cmkAuditor:  cmkAuditor,
	}

	if clientsFactory != nil {
		reconciler.registry = clientsFactory.Registry()
	} else {
		log.Warn(ctx, "Creating CryptoReconciler without registry client")
	}

	managerOpts := []orbital.ManagerOptsFunc{
		orbital.WithTargets(targets),
		orbital.WithJobConfirmFunc(reconciler.confirmJob),
		orbital.WithJobDoneEventFunc(reconciler.jobTerminationFunc),
		orbital.WithJobFailedEventFunc(reconciler.jobTerminationFunc),
		orbital.WithJobCanceledEventFunc(reconciler.jobTerminationFunc),
	}

	manager, err := orbital.NewManager(orbRepo, reconciler.resolveTasks(), managerOpts...)
	if err != nil {
		return nil, errs.Wrapf(err, "failed to create orbital manager")
	}

	manager.Config.MaxReconcileCount = getMaxReconcileCount(&cfg.EventProcessor)
	for _, opt := range opts {
		opt(manager)
	}

	reconciler.manager = manager

	systemResolver := &SystemTaskInfoResolver{
		repo:        repository,
		svcRegistry: svcRegistry,
		targets:     targetMap,
	}

	//keyResolver := &KeyTaskInfoResolver{
	//	repo:        repository,
	//	targets:     targetMap,
	//}

	jobHandlerMap := map[JobType]JobHandler{
		JobTypeSystemLink:   NewSystemLinkJobHandler(repository, cmkAuditor, reconciler.registry, systemResolver),
		JobTypeSystemUnlink: NewSystemUnlinkJobHandler(repository, cmkAuditor, reconciler.registry, systemResolver),
		JobTypeSystemSwitch: NewSystemSwitchJobHandler(repository, cmkAuditor, reconciler.registry, systemResolver),
	}
	reconciler.jobHandlerMap = jobHandlerMap

	return reconciler, nil
}

// Start starts the orbital manager.
func (c *CryptoReconciler) Start(ctx context.Context) error {
	return c.manager.Start(ctx)
}

func (c *CryptoReconciler) CloseAmqpClients(ctx context.Context) {
	for _, initiator := range c.initiators {
		if amqpClient, ok := initiator.(*amqp.Client); ok {
			_ = amqpClient.Close(ctx)
		}
	}
}

func (c *CryptoReconciler) getHandlerByJobType(jobType JobType) (JobHandler, error) {
	handler, ok := c.jobHandlerMap[jobType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedTaskType, jobType)
	}
	return handler, nil
}

// resolveTasks is called to resolve tasks for a job.
func (c *CryptoReconciler) resolveTasks() orbital.TaskResolveFunc {
	return func(ctx context.Context, job orbital.Job, _ orbital.TaskResolverCursor) (orbital.TaskResolverResult, error) {
		handler := c.jobHandlerMap[JobType(job.Type)]
		if handler == nil {
			return orbital.TaskResolverResult{
				IsCanceled:           true,
				CanceledErrorMessage: fmt.Sprintf("unsupported job type: %s", job.Type),
			}, nil
		}

		tasks, err := handler.ResolveTasks(ctx, job)
		if err != nil {
			return orbital.TaskResolverResult{
				IsCanceled:           true,
				CanceledErrorMessage: fmt.Sprintf("failed to resolve tasks for job: %v", err),
			}, nil
		}

		if len(tasks) == 0 {
			return orbital.TaskResolverResult{
				IsCanceled:           true,
				CanceledErrorMessage: "no tasks resolved for the job",
			}, nil
		}

		return orbital.TaskResolverResult{
			TaskInfos: tasks,
			Done:      true,
		}, nil
	}
}

// confirmJob is called to confirm if a job can be processed.
func (c *CryptoReconciler) confirmJob(ctx context.Context, job orbital.Job) (orbital.JobConfirmResult, error) {
	handler := c.jobHandlerMap[JobType(job.Type)]
	if handler == nil {
		return orbital.JobConfirmResult{
			IsCanceled:           true,
			CanceledErrorMessage: fmt.Sprintf("unsupported job type: %s", job.Type),
		}, nil
	}

	result, err := handler.Confirm(ctx, job)
	if err != nil {
		return orbital.JobConfirmResult{
			IsCanceled:           true,
			CanceledErrorMessage: fmt.Sprintf("failed to confirm job: %v", err),
		}, nil
	}

	return result, nil
}

// jobTerminationFunc is called when a job is terminated.
func (c *CryptoReconciler) jobTerminationFunc(ctx context.Context, job orbital.Job) error {
	handler := c.jobHandlerMap[JobType(job.Type)]
	if handler == nil {
		log.Warn(ctx, fmt.Sprintf("No job handler found for job type: %s", job.Type))
		return nil
	}

	err := handler.Terminate(ctx, job)
	if err != nil {
		return err
	}

	return nil
}

//if jobData.Trigger == constants.SystemActionDecommission {
//	_, err = c.registry.Mapping().UnmapSystemFromTenant(ctx, &mappingv1.UnmapSystemFromTenantRequest{
//		ExternalId: system.Identifier,
//		Type:       strings.ToLower(system.Type),
//		TenantId:   jobData.TenantID,
//	})
//	if err != nil {
//		return fmt.Errorf("failed to unmap system from tenant: %w", err)
//	}
//}

func getMaxReconcileCount(cfg *config.EventProcessor) uint64 {
	if cfg.MaxReconcileCount <= 0 {
		return defaultMaxReconcileCount
	}

	return cfg.MaxReconcileCount
}

// createTargets initializes the AMQP clients for each manager target defined in the orbital configuration.
func createAMQPTargets(ctx context.Context, cfg *config.EventProcessor) (map[string]orbital.TargetManager, error) {
	targets := make(map[string]orbital.TargetManager)

	options, err := getAMQPOptions(cfg)
	if err != nil {
		return nil, err
	}

	for _, r := range cfg.Targets {
		connInfo := amqp.ConnectionInfo{
			URL:    r.AMQP.URL,
			Target: r.AMQP.Target,
			Source: r.AMQP.Source,
		}

		client, err := amqp.NewClient(ctx, &codec.Proto{}, connInfo, options...)
		if err != nil {
			return nil, fmt.Errorf("failed to create AMQP client for responder %s: %w", r.Region, err)
		}

		targets[r.Region] = orbital.TargetManager{
			Client: client,
		}
	}

	return targets, nil
}

func getAMQPOptions(cfg *config.EventProcessor) ([]amqp.ClientOption, error) {
	if cfg.SecretRef.Type != commoncfg.MTLSSecretType {
		return []amqp.ClientOption{}, nil
	}

	tlsConfig, err := commoncfg.LoadMTLSConfig(&cfg.SecretRef.MTLS)
	if err != nil {
		return nil, errs.Wrap(config.ErrLoadMTLSConfig, err)
	}

	return []amqp.ClientOption{
		func(o *goAmqp.ConnOptions) error {
			o.TLSConfig = tlsConfig
			o.SASLType = goAmqp.SASLTypeExternal("")

			return nil
		},
	}, nil
}
