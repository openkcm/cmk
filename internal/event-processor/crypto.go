package eventprocessor

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/openkcm/orbital/codec"

	_ "github.com/lib/pq" // Import PostgreSQL driver to initialize the database connection

	goAmqp "github.com/Azure/go-amqp"
	orbsql "github.com/openkcm/orbital/store/sql"
	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
	keystoreopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"
	protoPkg "google.golang.org/protobuf/proto"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/db/dsn"
	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/internal/event-processor/proto"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	cmkcontext "github.com/openkcm/cmk-core/utils/context"
)

const (
	// defaultMaxReconcileCount If want to limit the reconcile period for one task to one day,
	// need maxReconcileCount = 18, as there is an exponential backoff for retries,
	// starting with 10s and limiting at 10240s.
	defaultMaxReconcileCount = 18
)

var (
	ErrTargetNotConfigured       = errors.New("target not configured for region")
	ErrUnsupportedTaskType       = errors.New("unsupported task type")
	ErrKeyAccessMetadataNotFound = errors.New("key access metadata not found for system region")
	ErrPluginNotFound            = errors.New("plugin not found for key provider")
)

type Option func(manager *orbital.Manager)

func WithMaxReconcileCount(n int64) Option {
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
	repo          repo.Repo
	manager       *orbital.Manager
	targets       map[string]struct{}
	pluginCatalog *plugincatalog.Catalog
}

// NewCryptoReconciler creates a new CryptoReconciler instance.
func NewCryptoReconciler(
	ctx context.Context,
	cfg *config.Config,
	repository repo.Repo,
	pluginCatalog *plugincatalog.Catalog,
	opts ...Option,
) (*CryptoReconciler, error) {
	db, err := initOrbitalSchema(ctx, cfg.Database)
	if err != nil {
		return nil, err
	}

	store, err := orbsql.New(ctx, db)
	if err != nil {
		return nil, errs.Wrapf(err, "failed to create orbital store")
	}

	orbRepo := orbital.NewRepository(store)

	targets, err := createTargets(ctx, &cfg.EventProcessor)
	if err != nil {
		return nil, errs.Wrapf(err, "failed to create targets")
	}

	targetMap := make(map[string]struct{})
	for region := range targets {
		targetMap[region] = struct{}{}
	}

	reconciler := &CryptoReconciler{
		repo:          repository,
		targets:       targetMap,
		pluginCatalog: pluginCatalog,
	}

	managerOpts := []orbital.ManagerOptsFunc{
		orbital.WithTargetClients(targets),
		orbital.WithJobConfirmFunc(reconciler.confirmJob),
		orbital.WithJobDoneEventFunc(reconciler.jobTerminationFunc),
		orbital.WithJobFailedEventFunc(reconciler.jobTerminationFunc),
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

	return reconciler, nil
}

// Start starts the orbital manager.
func (c *CryptoReconciler) Start(ctx context.Context) error {
	return c.manager.Start(ctx)
}

func (c *CryptoReconciler) createJob(ctx context.Context, data []byte, eventType string) (orbital.Job, error) {
	job := orbital.NewJob(eventType, data)

	return c.manager.PrepareJob(ctx, job)
}

// createTargets initializes the AMQP clients for each responder defined in the orbital configuration.
func createTargets(ctx context.Context, cfg *config.EventProcessor) (map[string]orbital.Initiator, error) {
	targets := make(map[string]orbital.Initiator)

	options, err := getAmqpOptions(cfg)
	if err != nil {
		return nil, err
	}

	for _, r := range cfg.Targets {
		connInfo := amqp.ConnectionInfo{
			URL:    r.AMQP.URL,
			Target: r.AMQP.Target,
			Source: r.AMQP.Source,
		}

		initiator, err := amqp.NewClient(ctx, &codec.Proto{}, connInfo, options...)
		if err != nil {
			return nil, fmt.Errorf("failed to create AMQP client for responder %s: %w", r.Region, err)
		}

		targets[r.Region] = initiator
	}

	return targets, nil
}

func getAmqpOptions(cfg *config.EventProcessor) ([]amqp.ClientOption, error) {
	options := make([]amqp.ClientOption, 0)

	if cfg.SecretRef.Type != commoncfg.MTLSSecretType {
		return options, nil
	}

	tlsConfig, err := commoncfg.LoadMTLSConfig(&cfg.SecretRef.MTLS)
	if err != nil {
		return nil, errs.Wrap(config.ErrLoadMTLSConfig, err)
	}

	options = append(options,
		func(o *goAmqp.ConnOptions) error {
			o.TLSConfig = tlsConfig
			o.SASLType = goAmqp.SASLTypeExternal("")

			return nil
		})

	return options, nil
}

// resolveTasks is called to resolve tasks for a job.
func (c *CryptoReconciler) resolveTasks() orbital.TaskResolveFunc {
	return func(ctx context.Context, job orbital.Job, _ orbital.TaskResolverCursor) (orbital.TaskResolverResult, error) {
		var (
			result []orbital.TaskInfo
			err    error
		)

		taskType := proto.TaskType(proto.TaskType_value[job.Type])
		if isKeyActionTask(taskType) {
			result, err = c.getKeyTaskInfo(ctx, job.Data, taskType)
			if err != nil {
				return orbital.TaskResolverResult{
					IsCanceled:           true,
					CanceledErrorMessage: fmt.Sprintf("failed to get key task info: %v", err),
				}, nil
			}
		} else {
			result, err = c.getSystemTaskInfo(ctx, job.Data, taskType)
			if err != nil {
				return orbital.TaskResolverResult{
					IsCanceled:           true,
					CanceledErrorMessage: fmt.Sprintf("failed to get system task info: %v", err),
				}, nil
			}
		}

		return orbital.TaskResolverResult{
			TaskInfos: result,
			Done:      true,
		}, nil
	}
}

func isKeyActionTask(taskType proto.TaskType) bool {
	return taskType == proto.TaskType_KEY_DELETE ||
		taskType == proto.TaskType_KEY_ROTATE ||
		taskType == proto.TaskType_KEY_DISABLE ||
		taskType == proto.TaskType_KEY_ENABLE
}

func isSystemActionTask(taskType proto.TaskType) bool {
	return taskType == proto.TaskType_SYSTEM_LINK ||
		taskType == proto.TaskType_SYSTEM_UNLINK ||
		taskType == proto.TaskType_SYSTEM_SWITCH
}

func (c *CryptoReconciler) getKeyTaskInfo(
	ctx context.Context,
	jobData []byte,
	taskType proto.TaskType,
) ([]orbital.TaskInfo, error) {
	var data KeyActionJobData

	err := json.Unmarshal(jobData, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	tenant, err := c.getTenantByID(ctx, data.TenantID)
	if err != nil {
		return nil, err
	}

	result := make([]orbital.TaskInfo, 0, len(c.targets))

	for target := range c.targets {
		taskData := &proto.Data{
			TaskType: taskType,
			Data: &proto.Data_KeyAction{
				KeyAction: &proto.KeyAction{
					KeyId:     data.KeyID,
					TenantId:  tenant.ID,
					CmkRegion: tenant.Region,
				},
			},
		}

		taskDataBytes, err := protoPkg.Marshal(taskData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal task data: %w", err)
		}

		result = append(result, orbital.TaskInfo{
			Target: target,
			Data:   taskDataBytes,
			Type:   taskType.String(),
		})
	}

	return result, nil
}

//nolint:funlen,cyclop
func (c *CryptoReconciler) getSystemTaskInfo(
	ctx context.Context,
	jobData []byte,
	taskType proto.TaskType,
) ([]orbital.TaskInfo, error) {
	var data SystemActionJobData

	err := json.Unmarshal(jobData, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	tenant, err := c.getTenantByID(ctx, data.TenantID)
	if err != nil {
		return nil, err
	}

	system, err := c.getSystemByID(ctx, data.SystemID, data.TenantID)
	if err != nil {
		return nil, err
	}

	_, ok := c.targets[system.Region]
	if !ok {
		return nil, errs.Wrapf(ErrTargetNotConfigured, system.Region)
	}

	key, err := c.getKeyByKeyID(ctx, data.KeyID, data.TenantID)
	if err != nil {
		return nil, err
	}

	keyAccessMetadata, err := c.getKeyAccessMetadata(ctx, *key, system.Region)
	if err != nil {
		return nil, err
	}

	var keyIDFrom, keyIDTo string

	//nolint:exhaustive
	switch taskType {
	case proto.TaskType_SYSTEM_LINK:
		keyIDTo = data.KeyID
	case proto.TaskType_SYSTEM_UNLINK:
		keyIDFrom = data.KeyID
	default:
	}

	result := make([]orbital.TaskInfo, 0, 1)
	taskData := &proto.Data{
		TaskType: taskType,
		Data: &proto.Data_SystemAction{
			SystemAction: &proto.SystemAction{
				SystemId:          system.Identifier,
				SystemRegion:      system.Region,
				SystemType:        strings.ToLower(system.Type),
				KeyIdFrom:         keyIDFrom,
				KeyIdTo:           keyIDTo,
				KeyProvider:       strings.ToLower(key.Provider),
				TenantId:          tenant.ID,
				CmkRegion:         tenant.Region,
				KeyAccessMetaData: keyAccessMetadata,
			},
		},
	}

	taskDataBytes, err := protoPkg.Marshal(taskData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task data: %w", err)
	}

	result = append(result, orbital.TaskInfo{
		Target: system.Region,
		Data:   taskDataBytes,
		Type:   taskType.String(),
	})

	return result, nil
}

func (c *CryptoReconciler) getKeyAccessMetadata(
	ctx context.Context,
	key model.Key,
	systemRegion string,
) ([]byte, error) {
	plugin := c.pluginCatalog.LookupByTypeAndName(keystoreopv1.Type, key.Provider)
	if plugin == nil {
		return nil, ErrPluginNotFound
	}

	cryptoAccessData, err := keystoreopv1.NewKeystoreInstanceKeyOperationClient(plugin.ClientConnection()).
		TransformCryptoAccessData(
			ctx,
			&keystoreopv1.TransformCryptoAccessDataRequest{
				NativeKeyId: *key.NativeID,
				AccessData:  key.CryptoAccessData,
			})
	if err != nil {
		return nil, err
	}

	keyAccessMetadata, ok := cryptoAccessData.GetTransformedAccessData()[systemRegion]
	if !ok {
		return nil, ErrKeyAccessMetadataNotFound
	}

	return keyAccessMetadata, nil
}

// confirmJob is called to confirm if a job can be processed.
func (c *CryptoReconciler) confirmJob(ctx context.Context, job orbital.Job) (orbital.JobConfirmResult, error) {
	taskType := proto.TaskType(proto.TaskType_value[job.Type])

	// if key event nothing to check for confirmation
	if isKeyActionTask(taskType) {
		return orbital.JobConfirmResult{
			Done: true,
		}, nil
	}

	if !isSystemActionTask(taskType) {
		return orbital.JobConfirmResult{}, errs.Wrapf(ErrUnsupportedTaskType, taskType.String())
	}

	var systemJobData SystemActionJobData

	err := json.Unmarshal(job.Data, &systemJobData)
	if err != nil {
		return orbital.JobConfirmResult{}, err
	}

	system, err := c.getSystemByID(ctx, systemJobData.SystemID, systemJobData.TenantID)
	if err != nil {
		return orbital.JobConfirmResult{
			Done:                 false,
			CanceledErrorMessage: err.Error(),
		}, err
	}

	if system.Status != cmkapi.SystemStatusPROCESSING {
		return orbital.JobConfirmResult{}, nil
	}

	return orbital.JobConfirmResult{
		Done: true,
	}, nil
}

// jobTerminationFunc is called when a job is terminated.
func (c *CryptoReconciler) jobTerminationFunc(ctx context.Context, job orbital.Job) error {
	taskType := proto.TaskType(proto.TaskType_value[job.Type])
	status := cmkapi.SystemStatusFAILED

	var jobData SystemActionJobData

	//nolint:exhaustive
	switch taskType {
	case proto.TaskType_SYSTEM_LINK:
		if job.Status == orbital.JobStatusDone {
			status = cmkapi.SystemStatusCONNECTED
		}
	case proto.TaskType_SYSTEM_UNLINK:
		if job.Status == orbital.JobStatusDone {
			status = cmkapi.SystemStatusDISCONNECTED
		}
	default:
		return nil
	}

	err := json.Unmarshal(job.Data, &jobData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal system action job data: %w", err)
	}

	return c.updateSystemsStatus(ctx, status, jobData.SystemID, jobData.TenantID)
}

// updateSystemsStatus updates the status of systems in a transaction
func (c *CryptoReconciler) updateSystemsStatus(
	ctx context.Context,
	status cmkapi.SystemStatus,
	systemID string,
	tenantID string,
) error {
	system, err := c.getSystemByID(ctx, systemID, tenantID)
	if err != nil {
		return err
	}

	system.Status = status

	ck := repo.NewCompositeKey().Where(repo.IDField, system.ID)
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(ck),
	).Update(repo.StatusField)

	cmkContext := cmkcontext.CreateTenantContext(ctx, tenantID)

	_, err = c.repo.Patch(cmkContext, system, *query)
	if err != nil {
		return fmt.Errorf("failed to update system %s status to %s: %w", system.ID, status, err)
	}

	return nil
}

func (c *CryptoReconciler) getSystemByID(ctx context.Context, systemID string, tenantID string) (*model.System, error) {
	cmkContext := cmkcontext.CreateTenantContext(ctx, tenantID)

	var system model.System

	ck := repo.NewCompositeKey().Where(repo.IDField, systemID)
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(ck),
	)

	_, err := c.repo.First(cmkContext, &system, *query)
	if err != nil {
		return nil, err
	}

	return &system, nil
}

func (c *CryptoReconciler) getKeyByKeyID(ctx context.Context, keyID string, tenantID string) (*model.Key, error) {
	cmkContext := cmkcontext.CreateTenantContext(ctx, tenantID)

	var key model.Key

	ck := repo.NewCompositeKey().Where(repo.IDField, keyID)
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(ck),
	)

	_, err := c.repo.First(cmkContext, &key, *query)
	if err != nil {
		return nil, fmt.Errorf("failed to get key by ID %s: %w", keyID, err)
	}

	return &key, nil
}

func (c *CryptoReconciler) getTenantByID(ctx context.Context, tenantID string) (*model.Tenant, error) {
	cmkContext := cmkcontext.CreateTenantContext(ctx, tenantID)

	var tenant model.Tenant

	_, err := c.repo.First(cmkContext, &tenant, *repo.NewQuery().
		Where(
			repo.NewCompositeKeyGroup(
				repo.NewCompositeKey().
					Where(repo.IDField, tenantID),
			),
		),
	)
	if err != nil {
		return nil, err
	}

	return &tenant, nil
}

func initOrbitalSchema(ctx context.Context, cfg config.Database) (*sql.DB, error) {
	baseDSN, err := dsn.FromDBConfig(cfg)
	if err != nil {
		return nil, err
	}

	orbitalDSN := baseDSN + " search_path=orbital,public sslmode=disable"

	orbitalDB, err := sql.Open("postgres", orbitalDSN)
	if err != nil {
		return nil, fmt.Errorf("orbit pool: %w", err)
	}

	_, err = orbitalDB.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS orbital")
	if err != nil {
		return nil, fmt.Errorf("ensure schema: %w", err)
	}

	return orbitalDB, nil
}

func getMaxReconcileCount(cfg *config.EventProcessor) int64 {
	if cfg.MaxReconcileCount <= 0 {
		return defaultMaxReconcileCount
	}

	return cfg.MaxReconcileCount
}
