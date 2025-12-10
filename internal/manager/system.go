package manager

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/openkcm/orbital"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
	slogctx "github.com/veqryn/slog-context"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/clients"
	"github.tools.sap/kms/cmk/internal/clients/registry"
	"github.tools.sap/kms/cmk/internal/clients/registry/systems"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/errs"
	eventprocessor "github.tools.sap/kms/cmk/internal/event-processor"
	"github.tools.sap/kms/cmk/internal/event-processor/proto"
	"github.tools.sap/kms/cmk/internal/log"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
	"github.tools.sap/kms/cmk/utils/ptr"
)

type System interface {
	GetAllSystems(ctx context.Context, params repo.QueryMapper) ([]*model.System, int, error)
	GetSystemByID(ctx context.Context, keyConfigID uuid.UUID) (*model.System, error)
	GetSystemLinkByID(ctx context.Context, keyConfigID uuid.UUID) (*uuid.UUID, error)
	PatchSystemLinkByID(ctx context.Context, systemID uuid.UUID, patchSystem cmkapi.SystemPatch) (*model.System, error)
	DeleteSystemLinkByID(ctx context.Context, systemID uuid.UUID) error
	RefreshSystemsData(ctx context.Context) bool
	CancelSystemAction(ctx context.Context, systemID uuid.UUID) error
}

type SystemManager struct {
	repo             repo.Repo
	registry         registry.Service
	reconciler       *eventprocessor.CryptoReconciler
	sisClient        *SystemInformation
	KeyConfigManager *KeyConfigManager
}

type SystemFilter struct {
	KeyConfigID uuid.UUID
	Region      string
	Type        string
	Skip        int
	Top         int
}

var SystemEvents = []string{
	proto.TaskType_SYSTEM_LINK.String(),
	proto.TaskType_SYSTEM_UNLINK.String(),
	proto.TaskType_SYSTEM_SWITCH.String(),
}

var _ repo.QueryMapper = (*SystemFilter)(nil) // Assert interface impl

func (s SystemFilter) GetQuery() *repo.Query {
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
	reconciler *eventprocessor.CryptoReconciler,
	ctlg *plugincatalog.Catalog,
	cfg *config.Config,
	keyConfigManager *KeyConfigManager,
) *SystemManager {
	manager := &SystemManager{
		repo:             repository,
		reconciler:       reconciler,
		KeyConfigManager: keyConfigManager,
	}

	if clientsFactory != nil {
		manager.registry = clientsFactory.Registry()
	} else {
		log.Warn(ctx, "Creating SystemManager without registry client")
	}

	sisClient, err := NewSystemInformationManager(repository, ctlg, &cfg.ContextModels.System)
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

	systems, count, err := repo.ListSystemWithProperties(ctx, m.repo, params.GetQuery())
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

func (m *SystemManager) CancelSystemAction(ctx context.Context, systemID uuid.UUID) error {
	event, err := m.reconciler.GetLastEvent(ctx, systemID.String())
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

func (m *SystemManager) GetSystemByID(ctx context.Context, systemID uuid.UUID) (*model.System, error) {
	system, err := repo.GetSystemByIDWithProperties(ctx, m.repo, systemID, repo.NewQuery())
	if err != nil {
		return nil, errs.Wrap(ErrGettingSystemByID, err)
	}

	// Check authorization for the system's key configuration (if exists)
	// Note: If the system is not linked to any key configuration, it is accessible to all users
	if system.KeyConfigurationID != nil {
		err = m.KeyConfigManager.CheckKeyConfigGroupMembership(ctx, *system.KeyConfigurationID)
		if err != nil {
			return nil, err
		}

		return system, nil
	}

	return system, nil
}

func (m *SystemManager) GetSystemLinkByID(ctx context.Context, systemID uuid.UUID) (*uuid.UUID, error) {
	system := &model.System{ID: systemID}

	_, err := m.repo.First(ctx, system, repo.Query{})
	if err != nil {
		return nil, errs.Wrap(ErrGettingSystemLinkByID, err)
	}

	keyConfigurationID := system.KeyConfigurationID
	if keyConfigurationID != nil {
		err = m.KeyConfigManager.CheckKeyConfigGroupMembership(ctx, *keyConfigurationID)
		if err != nil {
			return nil, err
		}
	}

	if keyConfigurationID == nil {
		return nil, ErrKeyConfigurationIDNotFound
	}

	return keyConfigurationID, nil
}

//nolint:cyclop,funlen
func (m *SystemManager) PatchSystemLinkByID(
	ctx context.Context,
	systemID uuid.UUID,
	patchSystem cmkapi.SystemPatch,
) (*model.System, error) {
	if patchSystem.Retry != nil && *patchSystem.Retry {
		system, err := m.GetSystemByID(ctx, systemID)
		if err != nil {
			return nil, err
		}

		err = m.handleEventRetry(ctx, systemID)

		return system, err
	}

	var updatedSystem *model.System

	err := m.repo.Transaction(
		ctx, func(ctx context.Context, r repo.Repo) error {
			// First, get the system to check its current state
			// Note: GetSystemByID checks authorization for the SOURCE key configuration (if exists)
			system, err := m.GetSystemByID(ctx, systemID)
			if err != nil {
				return err
			}

			keyConfig := &model.KeyConfiguration{ID: patchSystem.KeyConfigurationID}

			// Check authorization for the TARGET key configuration
			// User must have access to BOTH source (checked above) and target to perform the link
			if patchSystem.KeyConfigurationID != uuid.Nil {
				err = m.KeyConfigManager.CheckKeyConfigGroupMembership(ctx, patchSystem.KeyConfigurationID)
				if err != nil {
					return err
				}
			}

			_, err = r.First(ctx, keyConfig, *repo.NewQuery())
			if err != nil {
				return errs.Wrap(ErrGettingKeyConfigByID, err)
			}

			if !ptr.IsNotNilUUID(keyConfig.PrimaryKeyID) {
				return ErrAddSystemNoPrimaryKey
			}

			if system.Status == cmkapi.SystemStatusPROCESSING || system.Status == cmkapi.SystemStatusFAILED {
				return ErrLinkSystemProcessingOrFailed
			}

			updatedSystem, err = m.updateSystems(ctx, *system, patchSystem)
			if err != nil {
				return err
			}

			event, err := m.eventSelector(ctx, r, updatedSystem, system.KeyConfigurationID, keyConfig)
			if err != nil {
				return err
			}

			err = m.reconciler.SendEvent(ctx, event)
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

func (m *SystemManager) DeleteSystemLinkByID(ctx context.Context, systemID uuid.UUID) error {
	var dbSystem *model.System

	err := m.repo.Transaction(
		ctx, func(ctx context.Context, r repo.Repo) error {
			system := &model.System{ID: systemID}

			_, err := r.First(ctx, system, repo.Query{})
			if err != nil {
				return errs.Wrap(ErrGettingSystemByID, err)
			}

			if !ptr.IsNotNilUUID(system.KeyConfigurationID) {
				return errs.Wrap(ErrUpdateSystem, ErrSystemNotLinked)
			}

			keyConfig := &model.KeyConfiguration{ID: *system.KeyConfigurationID}

			// Check authorization for the system's key configuration
			// User must have access to the key configuration to perform the unlink
			if system.KeyConfigurationID != nil {
				err = m.KeyConfigManager.CheckKeyConfigGroupMembership(ctx, *system.KeyConfigurationID)
				if err != nil {
					return err
				}
			}

			_, err = r.First(ctx, keyConfig, *repo.NewQuery())
			if err != nil {
				return errs.Wrap(ErrGettingKeyConfigByID, err)
			}

			system.KeyConfigurationID = nil

			if system.Status == cmkapi.SystemStatusPROCESSING || system.Status == cmkapi.SystemStatusFAILED {
				return ErrUnlinkSystemProcessingOrFailed
			}

			dbSystem = system

			err = m.reconciler.SendEvent(
				ctx, eventprocessor.Event{
					Name: proto.TaskType_SYSTEM_UNLINK.String(),
					Event: func(ctx context.Context) (orbital.Job, error) {
						return m.reconciler.SystemUnlink(ctx, dbSystem, keyConfig.PrimaryKeyID.String())
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

func (m *SystemManager) handleEventRetry(
	ctx context.Context,
	systemID uuid.UUID,
) error {
	system, err := m.GetSystemByID(ctx, systemID)
	if err != nil {
		return err
	}

	if system.Status != cmkapi.SystemStatusFAILED {
		return ErrRetryNonFailedSystem
	}

	lastJob, err := m.reconciler.GetLastEvent(ctx, systemID.String())
	if err != nil {
		return err
	}

	event := eventprocessor.Event{
		Name: lastJob.Type,
		Event: func(ctx context.Context) (orbital.Job, error) {
			var job orbital.Job

			err := m.repo.Transaction(
				ctx, func(ctx context.Context, r repo.Repo) error {
					system.Status = cmkapi.SystemStatusPROCESSING

					_, err := r.Patch(ctx, system, *repo.NewQuery())
					if err != nil {
						return err
					}

					job, err = m.reconciler.CreateJob(ctx, lastJob)

					return err
				},
			)

			return job, err
		},
	}

	err = m.reconciler.SendEvent(ctx, event)
	if err != nil {
		return err
	}

	return nil
}

func (m *SystemManager) eventSelector(
	ctx context.Context,
	r repo.Repo,
	updatedSystem *model.System,
	oldKeyConfigID *uuid.UUID,
	keyConfig *model.KeyConfiguration,
) (eventprocessor.Event, error) {
	if !ptr.IsNotNilUUID(oldKeyConfigID) {
		return eventprocessor.Event{
			Name: proto.TaskType_SYSTEM_LINK.String(),
			Event: func(ctx context.Context) (orbital.Job, error) {
				return m.reconciler.SystemLink(ctx, updatedSystem, keyConfig.PrimaryKeyID.String())
			},
		}, nil
	}

	if updatedSystem.KeyConfigurationID != oldKeyConfigID {
		oldKeyConfig := &model.KeyConfiguration{ID: *oldKeyConfigID}

		_, err := r.First(ctx, oldKeyConfig, *repo.NewQuery())
		if err != nil {
			return eventprocessor.Event{}, errs.Wrap(ErrGettingKeyConfigByID, err)
		}

		if ptr.IsNotNilUUID(oldKeyConfig.PrimaryKeyID) {
			return eventprocessor.Event{
				Name: proto.TaskType_SYSTEM_SWITCH.String(),
				Event: func(ctx context.Context) (orbital.Job, error) {
					return m.reconciler.SystemSwitch(
						ctx,
						updatedSystem,
						keyConfig.PrimaryKeyID.String(),
						oldKeyConfig.PrimaryKeyID.String(),
					)
				},
			}, nil
		}
	}

	return eventprocessor.Event{
		Name: proto.TaskType_SYSTEM_LINK.String(),
		Event: func(ctx context.Context) (orbital.Job, error) {
			return m.reconciler.SystemLink(ctx, updatedSystem, keyConfig.PrimaryKeyID.String())
		},
	}, nil
}

func (m *SystemManager) updateSystems(
	ctx context.Context,
	system model.System,
	patchSystem cmkapi.SystemPatch,
) (*model.System, error) {
	system.KeyConfigurationID = &patchSystem.KeyConfigurationID

	_, err := m.repo.Patch(ctx, &system, *repo.NewQuery())
	if err != nil {
		return nil, errs.Wrap(ErrUpdateSystem, err)
	}

	return &system, nil
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

	ctx = log.InjectSystem(ctx, newSystem)
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
			return m.repo.Transaction(
				ctx, func(ctx context.Context, r repo.Repo) error {
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
							_, err := r.Delete(ctx, &model.System{ID: dbSystem.ID}, query)
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
