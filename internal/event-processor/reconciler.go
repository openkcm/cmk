package eventprocessor

import (
	"context"
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
	mappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
	keystoreopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"
	protoPkg "google.golang.org/protobuf/proto"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/clients/registry"
	"github.com/openkcm/cmk/internal/clients/registry/systems"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/event-processor/proto"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
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

	targets, err := createTargets(ctx, &cfg.EventProcessor)
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

// createTargets initializes the AMQP clients for each manager target defined in the orbital configuration.
func createTargets(ctx context.Context, cfg *config.EventProcessor) (map[string]orbital.TargetManager, error) {
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
					CanceledErrorMessage: GetOrbitalError(ctx, err),
				}, nil
			}
		} else {
			result, err = c.getSystemTaskInfo(ctx, job.Data, taskType)
			if err != nil {
				return orbital.TaskResolverResult{
					IsCanceled:           true,
					CanceledErrorMessage: GetOrbitalError(ctx, err),
				}, nil
			}
		}

		if len(result) == 0 {
			return orbital.TaskResolverResult{
				IsCanceled:           true,
				CanceledErrorMessage: "no tasks to process for job",
			}, nil
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
		taskType == proto.TaskType_KEY_ENABLE ||
		taskType == proto.TaskType_KEY_DETACH
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

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)

	var targets map[string]struct{}
	switch taskType {
	case proto.TaskType_KEY_ENABLE, proto.TaskType_KEY_DISABLE:
		regions, err := c.getRegionsByKeyID(ctx, data.KeyID)
		if err != nil {
			return nil, err
		}
		if len(regions) == 0 {
			return nil, ErrNoConnectedRegionsForKey
		}
		targets = regions
	default:
		targets = c.targets
	}

	result := make([]orbital.TaskInfo, 0, len(targets))

	for target := range targets {
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

// getRegionsByKeyID gets all distinct regions with CONNECTED systems for a given key ID.
func (c *CryptoReconciler) getRegionsByKeyID(ctx context.Context, keyID string) (map[string]struct{}, error) {
	key := &model.Key{}
	_, err := c.repo.First(ctx, key, *repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(repo.IDField, keyID),
		),
	))
	if err != nil {
		return nil, fmt.Errorf("failed to get key by ID %s: %w", keyID, err)
	}

	regions := make(map[string]struct{})

	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(repo.KeyConfigIDField, key.KeyConfigurationID),
		),
	)
	err = repo.ProcessInBatch(ctx, c.repo, query, repo.DefaultLimit, func(systems []*model.System) error {
		for _, system := range systems {
			if system.Status == cmkapi.SystemStatusCONNECTED {
				if _, ok := c.targets[system.Region]; !ok {
					ctx := model.LogInjectSystem(ctx, system)
					log.Error(ctx,
						"skipping region for connected system as target is not configured", ErrUnsupportedRegion)
					continue
				}
				regions[system.Region] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get connected regions for key ID %s: %w", keyID, err)
	}

	return regions, nil
}

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

	systemActionData, err := c.getSystemActionData(ctx, taskType, data)
	if err != nil {
		return nil, err
	}

	result := make([]orbital.TaskInfo, 0, 1)
	taskData := &proto.Data{
		TaskType: taskType,
		Data: &proto.Data_SystemAction{
			SystemAction: &proto.SystemAction{
				SystemId:          systemActionData.system.Identifier,
				SystemRegion:      systemActionData.system.Region,
				SystemType:        strings.ToLower(systemActionData.system.Type),
				KeyIdFrom:         systemActionData.keyIDFrom,
				KeyIdTo:           systemActionData.keyIDTo,
				KeyProvider:       strings.ToLower(systemActionData.key.Provider),
				TenantId:          systemActionData.tenant.ID,
				TenantOwnerId:     systemActionData.tenant.OwnerID,
				TenantOwnerType:   systemActionData.tenant.OwnerType,
				CmkRegion:         systemActionData.tenant.Region,
				KeyAccessMetaData: systemActionData.keyAccessMetadata,
			},
		},
	}

	taskDataBytes, err := protoPkg.Marshal(taskData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task data: %w", err)
	}

	result = append(result, orbital.TaskInfo{
		Target: systemActionData.system.Region,
		Data:   taskDataBytes,
		Type:   taskType.String(),
	})

	return result, nil
}

type systemActionData struct {
	system            *model.System
	keyIDFrom         string
	keyIDTo           string
	key               *model.Key
	tenant            *model.Tenant
	keyAccessMetadata []byte
}

func (c *CryptoReconciler) getSystemActionData(ctx context.Context,
	taskType proto.TaskType, data SystemActionJobData,
) (*systemActionData, error) {
	tenant, err := c.getTenantByID(ctx, data.TenantID)
	if err != nil {
		return nil, err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)

	system, err := c.getSystemByID(ctx, data.SystemID)
	if err != nil {
		return nil, err
	}

	_, ok := c.targets[system.Region]
	if !ok {
		return nil, errs.Wrapf(ErrTargetNotConfigured, system.Region)
	}

	keyID := data.KeyIDTo
	if taskType == proto.TaskType_SYSTEM_UNLINK {
		keyID = data.KeyIDFrom
	}

	key, err := c.getKeyByKeyID(ctx, keyID)
	if err != nil {
		return nil, err
	}

	keyAccessMetadata, err := c.getKeyAccessMetadata(ctx, *key, system.Region)
	if err != nil {
		return nil, err
	}

	return &systemActionData{
		system:            system,
		keyIDFrom:         data.KeyIDFrom,
		keyIDTo:           data.KeyIDTo,
		key:               key,
		tenant:            tenant,
		keyAccessMetadata: keyAccessMetadata,
	}, nil
}

func (c *CryptoReconciler) getKeyAccessMetadata(
	ctx context.Context,
	key model.Key,
	systemRegion string,
) ([]byte, error) {
	plugin := c.svcRegistry.LookupByTypeAndName(keystoreopv1.Type, key.Provider)
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
	jobType, ok := proto.TaskType_value[job.Type]
	if !ok {
		return orbital.JobConfirmResult{}, errs.Wrapf(ErrUnsupportedTaskType, job.Type)
	}

	taskType := proto.TaskType(jobType)

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

	ctx = cmkcontext.CreateTenantContext(ctx, systemJobData.TenantID)

	system, err := c.getSystemByID(ctx, systemJobData.SystemID)
	if err != nil {
		return orbital.JobConfirmResult{
			Done:                 false,
			CanceledErrorMessage: err.Error(),
		}, err
	}

	if system.Status != cmkapi.SystemStatusPROCESSING {
		return orbital.JobConfirmResult{
			IsCanceled:           true,
			CanceledErrorMessage: fmt.Sprintf("system status is in %v instead of processing", system.Status),
		}, nil
	}

	return orbital.JobConfirmResult{
		Done: true,
	}, nil
}

// jobTerminationFunc is called when a job is terminated.
//
//nolint:cyclop
func (c *CryptoReconciler) jobTerminationFunc(ctx context.Context, job orbital.Job) error {
	taskType := proto.TaskType(proto.TaskType_value[job.Type])
	status := cmkapi.SystemStatusFAILED

	var jobData SystemActionJobData

	switch taskType {
	case proto.TaskType_SYSTEM_LINK, proto.TaskType_SYSTEM_SWITCH:
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

	ctx = cmkcontext.CreateTenantContext(ctx, jobData.TenantID)

	system, err := c.getSystemByID(ctx, jobData.SystemID)
	if err != nil {
		return err
	}

	jobDone := job.Status == orbital.JobStatusDone

	if jobDone {
		err = c.handleDoneSystemJob(ctx, job, system, taskType, jobData)
		if err != nil {
			return err
		}
	}

	if job.Status == orbital.JobStatusFailed {
		err := c.updateEventError(ctx, job.ExternalID, job.ErrorMessage)
		if err != nil {
			return err
		}
	}

	return c.updateSystemOnJobTerminate(ctx, system, jobData, taskType, status, jobDone)
}

func (c *CryptoReconciler) updateEventError(ctx context.Context, identifier string, errorMessage string) error {
	orbitalErr := ParseOrbitalError(errorMessage)
	event := &model.Event{Identifier: identifier}
	event.ErrorCode = orbitalErr.Code
	event.ErrorMessage = orbitalErr.Message

	_, err := c.repo.Patch(ctx, event, *repo.NewQuery())
	return err
}

func (c *CryptoReconciler) handleDoneSystemJob(
	ctx context.Context,
	job orbital.Job,
	system *model.System,
	taskType proto.TaskType,
	jobData SystemActionJobData,
) error {
	err := c.sendSystemAuditLogOnJobTerminate(ctx, system, jobData, taskType)
	if err != nil {
		log.Error(ctx, "failed to send audit log for successful system event", err)
	}

	err = c.setClientL1KeyClaimOnJobTerminate(ctx, jobData.TenantID, system, taskType)
	if err != nil {
		return fmt.Errorf("failed to set L1 key claim on job terminate: %w", err)
	}

	if jobData.Trigger == constants.SystemActionDecommission {
		_, err = c.registry.Mapping().UnmapSystemFromTenant(ctx, &mappingv1.UnmapSystemFromTenantRequest{
			ExternalId: system.Identifier,
			Type:       strings.ToLower(system.Type),
			TenantId:   jobData.TenantID,
		})
		if err != nil {
			return fmt.Errorf("failed to unmap system from tenant: %w", err)
		}
	}

	// Clean the event if it was successful as we no longer need to hold
	// previous state for cancel/retry actions
	_, err = c.repo.Delete(
		ctx,
		&model.Event{},
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(repo.NewCompositeKey().
			Where(repo.IdentifierField, job.ExternalID).
			Where(repo.TypeField, job.Type),
		)),
	)
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	return nil
}

// sendSystemAuditLogOnJobTerminate sends an audit log based on the task type and job data when a job is terminated.
func (c *CryptoReconciler) sendSystemAuditLogOnJobTerminate(
	ctx context.Context,
	system *model.System,
	jobData SystemActionJobData,
	taskType proto.TaskType,
) error {
	var err error

	switch taskType {
	case proto.TaskType_SYSTEM_LINK:
		err = c.cmkAuditor.SendCmkOnboardingAuditLog(ctx, jobData.KeyIDTo, system.Identifier)
	case proto.TaskType_SYSTEM_UNLINK:
		err = c.cmkAuditor.SendCmkOffboardingAuditLog(ctx, jobData.KeyIDFrom, system.Identifier)
	case proto.TaskType_SYSTEM_SWITCH:
		err = c.cmkAuditor.SendCmkSwitchAuditLog(ctx, system.Identifier, jobData.KeyIDFrom, jobData.KeyIDTo)
	default:
		return nil
	}

	return err
}

// updateSystemOnJobTerminate updates the status of systems in a transaction
func (c *CryptoReconciler) updateSystemOnJobTerminate(
	ctx context.Context,
	system *model.System,
	jobData SystemActionJobData,
	taskType proto.TaskType,
	status cmkapi.SystemStatus,
	updateKeyConfigID bool,
) error {
	system.Status = status

	var err error

	if updateKeyConfigID {
		switch taskType {
		case proto.TaskType_SYSTEM_LINK, proto.TaskType_SYSTEM_SWITCH:
			key, err := c.getKeyByKeyID(ctx, jobData.KeyIDTo)
			if err != nil {
				return err
			}

			system.KeyConfigurationID = &key.KeyConfigurationID
		case proto.TaskType_SYSTEM_UNLINK:
			system.KeyConfigurationID = nil
		default:
			return nil
		}
	}

	ck := repo.NewCompositeKey().Where(repo.IDField, system.ID)
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)).UpdateAll(true)

	_, err = c.repo.Patch(ctx, system, *query)
	if err != nil {
		return fmt.Errorf("failed to update system %s status and keyConfigID: %w", system.ID, err)
	}

	return nil
}

func (c *CryptoReconciler) setClientL1KeyClaimOnJobTerminate(
	ctx context.Context,
	tenant string,
	system *model.System,
	taskType proto.TaskType,
) error {
	if c.registry == nil {
		log.Warn(ctx, "Could not set L1 key claim - CryptoReconciler systems client is nil")
		return nil
	}

	var keyClaim bool

	switch taskType {
	case proto.TaskType_SYSTEM_LINK:
		keyClaim = true
	case proto.TaskType_SYSTEM_UNLINK:
		keyClaim = false
	default:
		return nil
	}

	err := c.registry.System().ExtendedUpdateSystemL1KeyClaim(ctx, systems.SystemFilter{
		ExternalID: system.Identifier,
		Region:     system.Region,
		TenantID:   tenant,
	}, keyClaim)
	if errors.Is(err, systems.ErrKeyClaimAlreadyActive) && keyClaim ||
		errors.Is(err, systems.ErrKeyClaimAlreadyInactive) && !keyClaim {
		// If the key claim is already set to the desired state, we can ignore the error.
		return nil
	} else if err != nil {
		return errs.Wrap(ErrSettingKeyClaim, err)
	}

	return nil
}

func (c *CryptoReconciler) getSystemByID(ctx context.Context, systemID string) (*model.System, error) {
	var system model.System

	ck := repo.NewCompositeKey().Where(repo.IDField, systemID)
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(ck),
	)

	_, err := c.repo.First(ctx, &system, *query)
	if err != nil {
		return nil, err
	}

	return &system, nil
}

func (c *CryptoReconciler) getKeyByKeyID(ctx context.Context, keyID string) (*model.Key, error) {
	var key model.Key

	ck := repo.NewCompositeKey().Where(repo.IDField, keyID)
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(ck),
	)

	_, err := c.repo.First(ctx, &key, *query)
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

func getMaxReconcileCount(cfg *config.EventProcessor) uint64 {
	if cfg.MaxReconcileCount <= 0 {
		return defaultMaxReconcileCount
	}

	return cfg.MaxReconcileCount
}
