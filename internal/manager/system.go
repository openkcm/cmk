package manager

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/openkcm/orbital"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/clients/registry"
	"github.com/openkcm/cmk/internal/clients/registry/systems"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/event-processor/proto"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

type System interface {
	GetAllSystems(ctx context.Context, params repo.QueryMapper) ([]*model.System, int, error)
	GetSystemByID(ctx context.Context, keyConfigID uuid.UUID) (*model.System, error)
	RefreshSystemsData(ctx context.Context) bool
	LinkSystemAction(ctx context.Context, systemID uuid.UUID, patchSystem cmkapi.SystemPatch) (*model.System, error)
	UnlinkSystemAction(ctx context.Context, systemID uuid.UUID, trigger string) error
	GetRecoveryActions(ctx context.Context, sytemID uuid.UUID) (cmkapi.SystemRecoveryAction, error)
	SendRecoveryActions(
		ctx context.Context,
		systemID uuid.UUID,
		action cmkapi.SystemRecoveryActionBodyAction,
	) error
}

type SystemManager struct {
	repo             repo.Repo
	registry         registry.Service
	eventFactory     *eventprocessor.EventFactory
	sisClient        *SystemInformation
	KeyConfigManager *KeyConfigManager
	ContextModelsCfg config.System
	user             User
}

type SystemFilter struct {
	KeyConfigID uuid.UUID
	Region      string
	Type        string
	Skip        int
	Top         int
	Count       bool
}

var SystemEvents = []string{
	proto.TaskType_SYSTEM_LINK.String(),
	proto.TaskType_SYSTEM_UNLINK.String(),
	proto.TaskType_SYSTEM_SWITCH.String(),
}

var _ repo.QueryMapper = (*SystemFilter)(nil) // Assert interface impl

func (s SystemFilter) GetPagination() repo.Pagination {
	return repo.Pagination{
		Skip:  s.Skip,
		Top:   s.Top,
		Count: s.Count,
	}
}

func (s SystemFilter) GetQuery(_ context.Context) *repo.Query {
	query := repo.NewQuery()

	ck := repo.NewCompositeKey()

	if s.KeyConfigID != uuid.Nil {
		ck = ck.Where(repo.KeyConfigIDField, s.KeyConfigID)
	}

	if s.Region != "" {
		ck = ck.Where(repo.RegionField, s.Region)
	}

	if s.Type != "" {
		ck = ck.Where(repo.TypeField, s.Type)
	}

	if len(ck.Conds) > 0 {
		query = query.Where(repo.NewCompositeKeyGroup(ck))
	}

	return query
}

func (s SystemFilter) GetUUID(field repo.QueryField) (uuid.UUID, error) {
	if field != repo.KeyConfigIDField {
		return uuid.Nil, ErrIncompatibleQueryField
	}

	if s.KeyConfigID == uuid.Nil {
		return uuid.Nil, nil
	}

	return s.KeyConfigID, nil
}

func (s SystemFilter) GetString(field repo.QueryField) (string, error) {
	var val string

	switch field {
	case repo.RegionField:
		val = s.Region
	case repo.TypeField:
		val = s.Type
	default:
		return "", ErrIncompatibleQueryField
	}

	return val, nil
}

func NewSystemManager(
	ctx context.Context,
	repository repo.Repo,
	clientsFactory clients.Factory,
	eventFactory *eventprocessor.EventFactory,
	svcRegistry *cmkpluginregistry.Registry,
	cfg *config.Config,
	keyConfigManager *KeyConfigManager,
	user User,
) *SystemManager {
	manager := &SystemManager{
		repo:             repository,
		eventFactory:     eventFactory,
		KeyConfigManager: keyConfigManager,
		user:             user,
	}

	if clientsFactory != nil {
		manager.registry = clientsFactory.Registry()
	} else {
		log.Warn(ctx, "Creating SystemManager without registry client")
	}

	manager.ContextModelsCfg = cfg.ContextModels.System

	sisClient, err := NewSystemInformationManager(repository, svcRegistry, &cfg.ContextModels.System)
	if err != nil {
		log.Warn(ctx, "Failed to create sis client", slog.String(slogctx.ErrKey, err.Error()))
	}

	manager.sisClient = sisClient

	return manager
}

func (m *SystemManager) GetAllSystems(
	ctx context.Context,
	params repo.QueryMapper,
) ([]*model.System, int, error) {
	keyConfigID, err := params.GetUUID(repo.KeyConfigIDField)
	if err != nil {
		return nil, 0, errs.Wrap(ErrQuerySystemList, err)
	}

	if keyConfigID != uuid.Nil {
		_, err := m.repo.First(
			ctx,
			&model.KeyConfiguration{ID: keyConfigID},
			*repo.NewQuery(),
		)
		if err != nil {
			return nil, 0, errs.Wrap(ErrKeyConfigurationNotFound, err)
		}
	}

	query := params.GetQuery(ctx)
	pagination := params.GetPagination()
	systems, count, err := repo.ListAndCountSystemWithProperties(ctx, m.repo, pagination, query)
	if err != nil {
		return nil, 0, errs.Wrap(ErrQuerySystemList, err)
	}

	return systems, count, nil
}

func (m *SystemManager) RefreshSystemsData(ctx context.Context) bool {
	if m.registry == nil {
		log.Warn(
			ctx, "Could not perform systems' data fetch from registry service - APIController systems client is nil",
		)

		return false
	}

	tenant, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		log.Error(ctx, "Could not extract tenant ID", err)

		return false
	}

	fetchedSystems, err := m.registry.System().GetSystemsWithFilter(ctx, systems.SystemFilter{TenantID: tenant})
	if err != nil && status.Code(err) != codes.NotFound {
		log.Error(ctx, "Could not fetch systems data from registry service", err)

		return false
	}

	for _, fetchedSystem := range fetchedSystems {
		err := m.createSystemIfNotExists(ctx, fetchedSystem)
		if err != nil {
			log.Error(ctx, "Could not save systems", err)
			return false
		}
	}

	// Remove systems that no longer exist in registry
	err = m.removeSystemsNotInRegistry(ctx, fetchedSystems)
	if err != nil {
		log.Error(ctx, "Could not remove stale systems", err)
		return false
	}

	return true
}

func (m *SystemManager) GetRecoveryActions(
	ctx context.Context,
	systemID uuid.UUID,
) (cmkapi.SystemRecoveryAction, error) {
	system, err := m.GetSystemByID(ctx, systemID)
	if err != nil {
		return cmkapi.SystemRecoveryAction{}, err
	}

	if system.Status != cmkapi.SystemStatusFAILED {
		return cmkapi.SystemRecoveryAction{
			CanRetry:  false,
			CanCancel: false,
		}, nil
	}

	// If there are no entries on last event for this system
	// cancel and retry are not possible
	lastEvent, err := m.eventFactory.GetLastEvent(ctx, systemID.String())
	if err != nil {
		return cmkapi.SystemRecoveryAction{
			CanRetry:  false,
			CanCancel: false,
		}, err
	}

	// Determine retry and cancel permissions based on event type
	canRetry, canCancel := m.determineRecoveryPermissions(ctx, lastEvent)

	return cmkapi.SystemRecoveryAction{
		CanRetry:  canRetry,
		CanCancel: canCancel,
	}, nil
}

func (m *SystemManager) SendRecoveryActions(
	ctx context.Context,
	systemID uuid.UUID,
	action cmkapi.SystemRecoveryActionBodyAction,
) error {
	switch action {
	case cmkapi.SystemRecoveryActionBodyActionCANCEL:
		return m.cancelSystemAction(ctx, systemID)
	case cmkapi.SystemRecoveryActionBodyActionRETRY:
		return m.retrySystemAction(ctx, systemID)
	default:
		return ErrUnsupportedSystemAction
	}
}

func (m *SystemManager) GetSystemByID(ctx context.Context, systemID uuid.UUID) (*model.System, error) {
	system, err := repo.GetSystemByIDWithProperties(ctx, m.repo, systemID, repo.NewQuery())
	if err != nil {
		return nil, errs.Wrap(ErrGettingSystemByID, err)
	}

	// Check authorization for the system's key configuration (if exists)
	// Note: If the system is not linked to any key configuration, it is accessible to all users
	_, err = m.user.HasSystemAccess(ctx, authz.ActionRead, system)
	if err != nil {
		return nil, err
	}

	return system, nil
}

//nolint:cyclop, funlen
func (m *SystemManager) LinkSystemAction(
	ctx context.Context,
	systemID uuid.UUID,
	patchSystem cmkapi.SystemPatch,
) (*model.System, error) {
	var updatedSystem *model.System

	err := m.repo.Transaction(ctx, func(ctx context.Context) error {
		// First, get the system to check its current state
		// Note: GetSystemByID checks authorization for the SOURCE key configuration (if exists)
		system, err := m.GetSystemByID(ctx, systemID)
		if err != nil {
			return err
		}

		updatedSystem = system
		keyConfig := &model.KeyConfiguration{ID: patchSystem.KeyConfigurationID}

		// Check authorization for the TARGET key configuration
		// User must have access to BOTH source (checked above) and target to perform the link
		if patchSystem.KeyConfigurationID != uuid.Nil {
			_, err = m.user.HasSystemAccess(ctx, authz.ActionSystemModifyLink, system)
			if err != nil {
				return err
			}
		}

		_, err = m.repo.First(ctx, keyConfig, *repo.NewQuery())
		if err != nil {
			return errs.Wrap(ErrGettingKeyConfigByID, err)
		}

		// Check if primary key exists
		if !ptr.IsNotNilUUID(keyConfig.PrimaryKeyID) {
			return ErrConnectSystemNoPrimaryKey
		}

		pKey := &model.Key{ID: *keyConfig.PrimaryKeyID}
		_, err = m.repo.First(ctx, pKey, *repo.NewQuery())
		if err != nil {
			return errs.Wrap(ErrGettingKeyByID, err)
		}

		// Pre-check System key state.
		// Should fail if the key is not enabled
		if pKey.State != string(cmkapi.KeyStateENABLED) {
			return ErrConnectSystemNoPrimaryKey
		}

		if system.Status == cmkapi.SystemStatusPROCESSING || system.Status == cmkapi.SystemStatusFAILED {
			return ErrLinkSystemProcessingOrFailed
		}

		event, err := m.selectEvent(ctx, system, keyConfig)
		if err != nil {
			return err
		}

		err = m.eventFactory.SendEvent(ctx, event)
		if err != nil {
			return err
		}

		return nil
	},
	)
	if err != nil {
		return nil, errs.Wrap(ErrUpdateSystem, err)
	}

	return updatedSystem, nil
}

// UnlinkSystemAction unlinks a system.
// Trigger is used to determinate what triggered the system unlink
// By default is not set, it's only set for tenant decomission to trigger the unmap system
// whenever the event finishes
func (m *SystemManager) UnlinkSystemAction(ctx context.Context, systemID uuid.UUID, trigger string) error {
	var dbSystem *model.System

	err := m.repo.Transaction(ctx, func(ctx context.Context) error {
		system := &model.System{ID: systemID}

		_, err := m.repo.First(ctx, system, repo.Query{})
		if err != nil {
			return errs.Wrap(ErrGettingSystemByID, err)
		}

		if !ptr.IsNotNilUUID(system.KeyConfigurationID) {
			return errs.Wrap(ErrUpdateSystem, ErrSystemNotLinked)
		}

		keyConfig := &model.KeyConfiguration{ID: *system.KeyConfigurationID}

		// Check authorization for the system's key configuration
		// User must have access to the key configuration to perform the unlink
		_, err = m.user.HasSystemAccess(ctx, authz.ActionSystemModifyLink, system)
		if err != nil {
			return err
		}

		_, err = m.repo.First(ctx, keyConfig, *repo.NewQuery())
		if err != nil {
			return errs.Wrap(ErrGettingKeyConfigByID, err)
		}

		if system.Status == cmkapi.SystemStatusPROCESSING {
			return ErrUnlinkSystemProcessing
		}

		dbSystem = system

		err = m.eventFactory.SendEvent(
			ctx, eventprocessor.Event{
				Name: proto.TaskType_SYSTEM_UNLINK.String(),
				Event: func(ctx context.Context) (orbital.Job, error) {
					return m.eventFactory.SystemUnlink(ctx, dbSystem, keyConfig.PrimaryKeyID.String(), trigger)
				},
			},
		)
		if err != nil {
			return err
		}

		return nil
	},
	)
	if err != nil {
		return err
	}

	return nil
}

func (m *SystemManager) cancelSystemAction(ctx context.Context, systemID uuid.UUID) error {
	event, err := m.eventFactory.GetLastEvent(ctx, systemID.String())
	if err != nil {
		return err
	}

	_, err = m.repo.Patch(
		ctx, &model.System{
			ID:     systemID,
			Status: cmkapi.SystemStatus(event.PreviousItemStatus),
		}, *repo.NewQuery(),
	)

	return err
}

func (m *SystemManager) retrySystemAction(ctx context.Context, systemID uuid.UUID) error {
	system, err := m.GetSystemByID(ctx, systemID)
	if err != nil {
		return err
	}

	if system.Status != cmkapi.SystemStatusFAILED {
		return ErrRetryNonFailedSystem
	}

	lastJob, err := m.eventFactory.GetLastEvent(ctx, systemID.String())
	if err != nil {
		return err
	}

	event := eventprocessor.Event{
		Name: lastJob.Type,
		Event: func(ctx context.Context) (orbital.Job, error) {
			var job orbital.Job

			err := m.repo.Transaction(ctx, func(ctx context.Context) error {
				system.Status = cmkapi.SystemStatusPROCESSING

				_, err := m.repo.Patch(ctx, system, *repo.NewQuery())
				if err != nil {
					return err
				}

				job, err = m.eventFactory.CreateJob(ctx, lastJob)

				return err
			},
			)

			return job, err
		},
	}

	return m.eventFactory.SendEvent(ctx, event)
}

func (m *SystemManager) selectEvent(
	ctx context.Context,
	system *model.System,
	newKeyConfig *model.KeyConfiguration,
) (eventprocessor.Event, error) {
	oldKeyConfigID := system.KeyConfigurationID

	// If system doesn't have a link already, we send SYSTEM_LINK
	if !ptr.IsNotNilUUID(oldKeyConfigID) {
		return eventprocessor.Event{
			Name: proto.TaskType_SYSTEM_LINK.String(),
			Event: func(ctx context.Context) (orbital.Job, error) {
				return m.eventFactory.SystemLink(ctx, system, newKeyConfig.PrimaryKeyID.String())
			},
		}, nil
	}

	// If new key config ID don't match the old one it is a SYSTEM_SWITCH
	if newKeyConfig.ID.String() != oldKeyConfigID.String() {
		oldKeyConfig := &model.KeyConfiguration{ID: *oldKeyConfigID}

		_, err := m.repo.First(ctx, oldKeyConfig, *repo.NewQuery())
		if err != nil {
			return eventprocessor.Event{}, errs.Wrap(ErrGettingKeyConfigByID, err)
		}

		if ptr.IsNotNilUUID(oldKeyConfig.PrimaryKeyID) {
			return eventprocessor.Event{
				Name: proto.TaskType_SYSTEM_SWITCH.String(),
				Event: func(ctx context.Context) (orbital.Job, error) {
					return m.eventFactory.SystemSwitch(
						ctx,
						system,
						newKeyConfig.PrimaryKeyID.String(),
						oldKeyConfig.PrimaryKeyID.String(),
						"", // Empty trigger for regular switch actions
					)
				},
			}, nil
		}
	}

	return eventprocessor.Event{
		Name: proto.TaskType_SYSTEM_LINK.String(),
		Event: func(ctx context.Context) (orbital.Job, error) {
			return m.eventFactory.SystemLink(ctx, system, newKeyConfig.PrimaryKeyID.String())
		},
	}, nil
}

func (m *SystemManager) createSystemIfNotExists(ctx context.Context, newSystem *model.System) error {
	// Systems are identified by their ExternalID and Region - those fields can not be updated
	system := &model.System{}
	query := *repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().
				Where(
					repo.IdentifierField, newSystem.Identifier,
				).
				Where(
					repo.RegionField, newSystem.Region,
				),
		),
	)

	count, _ := m.repo.Count(ctx, system, query)
	if count > 0 {
		return nil
	}

	ctx = model.LogInjectSystem(ctx, newSystem)
	log.Info(ctx, "Found new system from registry, adding to CMK DB")

	err := m.repo.Create(ctx, newSystem)
	if err != nil {
		return errs.Wrap(ErrCreatingSystem, err)
	}

	err = m.sisClient.updateSystem(ctx, newSystem)
	if err != nil {
		log.Warn(ctx, "SIS Update Failed", log.ErrorAttr(err))
	}

	return nil
}

func (m *SystemManager) removeSystemsNotInRegistry(ctx context.Context, registrySystems []*model.System) error {
	// Build a map of (identifier:region) from registry for quick lookup
	registrySystemsMap := make(map[string]bool)

	for _, sys := range registrySystems {
		key := sys.Identifier + ":" + sys.Region
		registrySystemsMap[key] = true
	}

	// Process all systems in batches and delete those not in registry
	// Transaction is inside the batch callback to avoid holding locks for too long
	err := repo.ProcessInBatch(
		ctx, m.repo, repo.NewQuery(), repo.DefaultLimit, func(systems []*model.System) error {
			// Process each batch in a separate transaction
			return m.repo.Transaction(ctx, func(ctx context.Context) error {
				for _, dbSystem := range systems {
					key := dbSystem.Identifier + ":" + dbSystem.Region
					if !registrySystemsMap[key] {
						log.Info(ctx, "System no longer exists in registry, removing from CMK DB")

						query := *repo.NewQuery().Where(
							repo.NewCompositeKeyGroup(
								repo.NewCompositeKey().Where(repo.IDField, dbSystem.ID),
							),
						)

						// Delete the system (BeforeDelete hook will automatically delete associated properties)
						_, err := m.repo.Delete(ctx, &model.System{ID: dbSystem.ID}, query)
						if err != nil {
							log.Error(ctx, "Failed to delete system", err)
							return err
						}

						log.Info(ctx, "Successfully removed system from CMK DB")
					}
				}

				return nil
			},
			)
		},
	)
	if err != nil {
		return errs.Wrap(ErrUpdateSystem, err)
	}

	return nil
}

// determineRecoveryPermissions determines retry and cancel permissions
// based on event type and user authorization
func (m *SystemManager) determineRecoveryPermissions(
	ctx context.Context,
	event *model.Event,
) (bool, bool) {
	var canRetry, canCancel bool
	// Helper to check authorization
	checkAuth := func(getKeyID func(*eventprocessor.SystemActionJobData) string) bool {
		jobData, err := eventprocessor.GetSystemJobData(event)
		if err == nil {
			return m.hasKeyAdminAccess(ctx, getKeyID(&jobData))
		}
		return false
	}

	switch event.Type {
	case proto.TaskType_SYSTEM_UNLINK.String():
		// Retry allowed for Key Admins of the source KeyConfig (KeyIDFrom)
		canRetry = checkAuth(func(d *eventprocessor.SystemActionJobData) string { return d.KeyIDFrom })
		canCancel = true

	case proto.TaskType_SYSTEM_SWITCH.String():
		jobData, err := eventprocessor.GetSystemJobData(event)
		if err == nil {
			if jobData.Trigger == constants.KeyActionSetPrimary {
				// Make Primary Key: Retry allowed for Key Admins of target KeyConfig, cancel not allowed
				canRetry = m.hasKeyAdminAccess(ctx, jobData.KeyIDTo)
				canCancel = false
			} else {
				// Regular Switch: Retry allowed for Key Admins of source KeyConfig, cancel allowed
				canRetry = m.hasKeyAdminAccess(ctx, jobData.KeyIDFrom)
				canCancel = true
			}
		} else {
			canRetry = false
			canCancel = false
		}

	case proto.TaskType_SYSTEM_LINK.String():
		// Retry allowed for Key Admins of the target KeyConfig (KeyIDTo)
		canRetry = checkAuth(func(d *eventprocessor.SystemActionJobData) string { return d.KeyIDTo })
		canCancel = true

	default:
		canRetry = false
		canCancel = false
	}

	return canRetry, canCancel
}

// hasKeyAdminAccess checks if the current user has Key Admin permissions for the KeyConfig
// associated with the given keyID string
func (m *SystemManager) hasKeyAdminAccess(ctx context.Context, keyIDStr string) bool {
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		return false
	}

	key := &model.Key{ID: keyID}
	_, err = m.repo.First(ctx, key, *repo.NewQuery())
	if err != nil {
		return false
	}

	// Check if user has Key Admin access (ActionUpdate implies admin access)
	_, err = m.user.HasKeyAccess(ctx, authz.ActionUpdate, key.KeyConfigurationID)
	return err == nil
}
