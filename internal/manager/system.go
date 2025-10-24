package manager

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/openkcm/orbital"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/auditor"
	"github.com/openkcm/cmk-core/internal/clients"
	"github.com/openkcm/cmk-core/internal/clients/registry"
	"github.com/openkcm/cmk-core/internal/clients/registry/systems"
	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/constants"
	"github.com/openkcm/cmk-core/internal/errs"
	eventprocessor "github.com/openkcm/cmk-core/internal/event-processor"
	"github.com/openkcm/cmk-core/internal/log"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	cmkcontext "github.com/openkcm/cmk-core/utils/context"
	"github.com/openkcm/cmk-core/utils/ptr"
)

type System interface {
	GetAllSystems(ctx context.Context, filter SystemFilter) ([]*model.System, int, error)
	GetSystemByID(ctx context.Context, keyConfigID uuid.UUID) (*model.System, error)
	GetSystemLinkByID(ctx context.Context, keyConfigID uuid.UUID) (*uuid.UUID, error)
	PatchSystemLinkByID(ctx context.Context, systemID uuid.UUID, patchSystem model.System) (*model.System, error)
	DeleteSystemLinkByID(ctx context.Context, systemID uuid.UUID) error
	RefreshSystemsData(ctx context.Context) bool
	NewSystemFilter(request cmkapi.GetAllSystemsRequestObject) SystemFilter
}

type SystemManager struct {
	repo       repo.Repo
	registry   *registry.Service
	reconciler *eventprocessor.CryptoReconciler
	sisClient  *SystemInformation
	cmkAuditor *auditor.Auditor
}

type SystemFilter struct {
	KeyConfigID uuid.UUID
	Region      string
	Type        string
	Skip        int
	Top         int
}

func NewSystemManager(
	ctx context.Context,
	repository repo.Repo,
	clientsFactory *clients.Factory,
	reconciler *eventprocessor.CryptoReconciler,
	ctlg *plugincatalog.Catalog,
	cmkAuditor *auditor.Auditor,
	cfg *config.Config,
) *SystemManager {
	manager := &SystemManager{
		repo:       repository,
		reconciler: reconciler,
		cmkAuditor: cmkAuditor,
		registry:   clientsFactory.RegistryService(),
	}

	sisClient, err := NewSystemInformationManager(repository, ctlg, &cfg.System)
	if err != nil {
		log.Warn(ctx, "Failed to create sis client", slog.String(slogctx.ErrKey, err.Error()))
	}

	manager.sisClient = sisClient

	return manager
}

func (s SystemFilter) IsKeyConfigQuery() bool {
	return s.KeyConfigID != uuid.Nil
}

func (s SystemFilter) ApplyToQuery(query *repo.Query) *repo.Query {
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

func (*SystemManager) NewSystemFilter(request cmkapi.GetAllSystemsRequestObject) SystemFilter {
	var region string
	if request.Params.Region != nil {
		region = strings.ToUpper(*request.Params.Region)
	}

	var sysType string
	if request.Params.Type != nil {
		sysType = strings.ToUpper(*request.Params.Type)
	}

	return SystemFilter{
		KeyConfigID: ptr.GetSafeDeref(request.Params.KeyConfigurationID),
		Region:      region,
		Type:        sysType,
		Skip:        ptr.GetIntOrDefault(request.Params.Skip, constants.DefaultSkip),
		Top:         ptr.GetIntOrDefault(request.Params.Top, constants.DefaultTop),
	}
}

func (m *SystemManager) GetAllSystems(
	ctx context.Context,
	filter SystemFilter,
) ([]*model.System, int, error) {
	if filter.IsKeyConfigQuery() {
		_, err := m.repo.First(
			ctx,
			&model.KeyConfiguration{ID: filter.KeyConfigID},
			*repo.NewQuery(),
		)
		if err != nil {
			return nil, 0, errs.Wrap(ErrKeyConfigurationNotFound, err)
		}
	}

	query := repo.NewQuery().SetLimit(filter.Top).SetOffset(filter.Skip)
	query = filter.ApplyToQuery(query)

	systems, count, err := repo.ListSystemWithProperties(ctx, m.repo, query)
	if err != nil {
		return nil, 0, errs.Wrap(ErrQuerySystemList, err)
	}

	return systems, count, nil
}

func (m *SystemManager) RefreshSystemsData(ctx context.Context) bool {
	if m.registry == nil {
		log.Warn(ctx, "Could not perform systems' data fetch from registry service - APIController systems client is nil")
		return false
	}

	tenant, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		log.Error(ctx, "Could not extract tenant ID", err)

		return false
	}

	fetchedSystems, err := m.registry.System().GetSystemsWithFilter(ctx, systems.SystemFilter{TenantID: tenant})
	if err != nil {
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

	return true
}

func (m *SystemManager) GetSystemByID(ctx context.Context, systemID uuid.UUID) (*model.System, error) {
	system, err := repo.GetSystemByIDWithProperties(ctx, m.repo, systemID, repo.NewQuery())
	if err != nil {
		return nil, errs.Wrap(ErrGettingSystemByID, err)
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
	if keyConfigurationID == nil {
		return nil, ErrKeyConfigurationIDNotFound
	}

	return keyConfigurationID, nil
}

func (m *SystemManager) PatchSystemLinkByID(
	ctx context.Context,
	systemID uuid.UUID,
	patchSystem model.System,
) (*model.System, error) {
	var dbSystem *model.System

	err := m.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		keyConfig := &model.KeyConfiguration{ID: *patchSystem.KeyConfigurationID}

		_, err := r.First(ctx, keyConfig, *repo.NewQuery())
		if err != nil {
			return errs.Wrap(ErrGettingKeyConfigByID, err)
		}

		if !ptr.IsNotNilUUID(keyConfig.PrimaryKeyID) {
			return ErrAddSystemNoPrimaryKey
		}

		system := &model.System{ID: systemID}

		_, err = r.First(ctx, system, *repo.NewQuery())
		if err != nil {
			return errs.Wrap(ErrGettingSystemByID, err)
		}

		if system.Status == cmkapi.SystemStatusPROCESSING || system.Status == cmkapi.SystemStatusFAILED {
			return ErrLinkSystemProcessingOrFailed
		}

		system.Status = cmkapi.SystemStatusPROCESSING

		dbSystem, err = m.updateSystems(ctx, system, &patchSystem)
		if err != nil {
			return err
		}

		dbSystem = system

		err = m.setClientL1KeyClaim(ctx, dbSystem, true)
		if err != nil {
			return err
		}

		err = m.sendSystemEvent(ctx, system, keyConfig, true)
		if err != nil {
			return err
		}

		m.handlePatchSystemLinkAuditLogs(ctx, system, &patchSystem, keyConfig)

		return nil
	})
	if err != nil {
		return nil, errs.Wrap(ErrUpdateSystem, err)
	}

	return dbSystem, nil
}

func (m *SystemManager) DeleteSystemLinkByID(ctx context.Context, systemID uuid.UUID) error {
	var dbSystem *model.System

	err := m.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		system := &model.System{ID: systemID}

		_, err := r.First(ctx, system, repo.Query{})
		if err != nil {
			return errs.Wrap(ErrGettingSystemByID, err)
		}

		if !ptr.IsNotNilUUID(system.KeyConfigurationID) {
			return errs.Wrap(ErrUpdateSystem, ErrSystemNotLinked)
		}

		keyConfig := &model.KeyConfiguration{ID: *system.KeyConfigurationID}

		_, err = r.First(ctx, keyConfig, *repo.NewQuery())
		if err != nil {
			return errs.Wrap(ErrGettingKeyConfigByID, err)
		}

		err = m.updateSystemForUnlink(ctx, r, system)
		if err != nil {
			return err
		}

		dbSystem = system

		err = m.setClientL1KeyClaim(ctx, dbSystem, false)
		if err != nil {
			return err
		}

		err = m.sendSystemEvent(ctx, system, keyConfig, false)
		if err != nil {
			return err
		}

		m.sendCmkOffboardingAuditLog(ctx, keyConfig, systemID)

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (m *SystemManager) updateSystems(
	ctx context.Context,
	system *model.System,
	patchSystem *model.System,
) (*model.System, error) {
	system.KeyConfigurationID = patchSystem.KeyConfigurationID

	_, err := m.repo.Patch(ctx, system, *repo.NewQuery())
	if err != nil {
		return nil, errs.Wrap(ErrUpdateSystem, err)
	}

	return system, nil
}

func (m *SystemManager) setClientL1KeyClaim(
	ctx context.Context,
	system *model.System,
	keyClaim bool,
) error {
	if m.registry == nil {
		log.Warn(ctx, "Could not set L1 key claim - APIController systems client is nil")
		return nil
	}

	tenant, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return errs.Wrap(ErrUpdateSystem, err)
	}

	err = m.registry.System().UpdateSystemL1KeyClaim(ctx, systems.SystemFilter{
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

func (m *SystemManager) sendSystemEvent(
	ctx context.Context,
	system *model.System,
	keyConfig *model.KeyConfiguration,
	linking bool,
) error {
	if m.reconciler == nil {
		return errs.Wrapf(ErrEventSendingFailed, "reconciler is not initialized")
	}

	var (
		eventName    string
		reconcilerFn func(context.Context, string, string) (orbital.Job, error)
	)
	if linking {
		reconcilerFn = m.reconciler.SystemLink
		eventName = "System link"
	} else {
		reconcilerFn = m.reconciler.SystemUnlink
		eventName = "System unlink"
	}

	ctx = log.InjectSystemEvent(ctx, eventName, system, keyConfig)

	job, err := reconcilerFn(ctx, system.ID.String(), keyConfig.PrimaryKeyID.String())
	if err != nil {
		log.Info(ctx, "Failed to send event")
		return errs.Wrap(ErrEventSendingFailed, err)
	}

	log.Info(ctx, "Event Sent", slog.String("JobID", job.ID.String()))

	return nil
}

func (m *SystemManager) createSystemIfNotExists(ctx context.Context, newSystem *model.System) error {
	// Systems are identified by their ExternalID and Region - those fields can not be updated
	system := &model.System{}
	query := *repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().
				Where(
					repo.IdentifierField, newSystem.Identifier).
				Where(
					repo.RegionField, newSystem.Region),
		),
	)

	found, _ := m.repo.First(ctx, system, query)
	if found {
		return nil
	}

	ctx = log.InjectSystem(ctx, newSystem)
	log.Info(ctx, "Found new system from registry, adding to CMK db")

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

func (m *SystemManager) updateSystemForUnlink(
	ctx context.Context,
	r repo.Repo,
	system *model.System,
) error {
	system.KeyConfigurationID = nil

	if system.Status == cmkapi.SystemStatusPROCESSING || system.Status == cmkapi.SystemStatusFAILED {
		return ErrUnlinkSystemProcessingOrFailed
	}

	system.Status = cmkapi.SystemStatusPROCESSING

	_, err := r.Patch(
		ctx,
		system,
		*repo.NewQuery().UpdateAll(true),
	)
	if err != nil {
		return errs.Wrap(ErrUpdateSystem, err)
	}

	return nil
}

func (m *SystemManager) handlePatchSystemLinkAuditLogs(
	ctx context.Context,
	system *model.System,
	patchSystem *model.System,
	keyConfig *model.KeyConfiguration,
) {
	previousConfigurationID := system.KeyConfigurationID
	newConfigurationID := patchSystem.KeyConfigurationID

	if previousConfigurationID == nil {
		// System is being onboarded for the first time
		m.sendCmkOnboardingAuditLog(ctx, keyConfig, system.ID)
	} else if ptr.IsNotNilUUID(newConfigurationID) && *previousConfigurationID != *newConfigurationID {
		// System is switching from one key configuration to another
		m.sendCmkSwitchAuditLog(ctx, system.ID, *previousConfigurationID, *newConfigurationID)
	}
}

func (m *SystemManager) sendCmkOnboardingAuditLog(
	ctx context.Context,
	keyConfig *model.KeyConfiguration,
	systemID uuid.UUID,
) {
	err := m.cmkAuditor.SendCmkOnboardingAuditLog(ctx, keyConfig.PrimaryKeyID.String(), systemID.String())
	if err != nil {
		log.Error(ctx, "Failed to send audit log for CMK Onboard", err)
	}

	log.Info(ctx, "Audit log for CMK Onboard sent successfully")
}

func (m *SystemManager) sendCmkSwitchAuditLog(
	ctx context.Context,
	systemID uuid.UUID,
	oldKeyConfigID uuid.UUID,
	newKeyConfigID uuid.UUID,
) {
	err := m.cmkAuditor.SendCmkSwitchAuditLog(
		ctx,
		systemID.String(),
		oldKeyConfigID.String(),
		newKeyConfigID.String(),
	)
	if err != nil {
		log.Error(ctx, "Failed to send audit log for CMK Switch", err)
	}

	log.Info(ctx, "Audit log for CMK Switch sent successfully")
}

func (m *SystemManager) sendCmkOffboardingAuditLog(
	ctx context.Context,
	keyConfig *model.KeyConfiguration,
	systemID uuid.UUID,
) {
	err := m.cmkAuditor.SendCmkOffboardingAuditLog(ctx, keyConfig.PrimaryKeyID.String(), systemID.String())
	if err != nil {
		log.Error(ctx, "Failed to send audit log for CMK Offboard", err)
	}

	log.Info(ctx, "Audit log for CMK Offboard sent successfully")
}
