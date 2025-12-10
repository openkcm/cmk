package manager

import (
	"context"
	"errors"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/namespace"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/auditor"
	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/log"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

type Tenant interface {
	GetTenant(ctx context.Context) (*model.Tenant, error) // Get tenant from context
	ListTenantInfo(ctx context.Context, issuerURL *string, skip int, top int) ([]*model.Tenant, int, error)
	CreateTenant(ctx context.Context, tenant *model.Tenant) error
	OffboardTenant(ctx context.Context) (OffboardingResult, error)
	DeleteTenant(ctx context.Context) error
}

type TenantManager struct {
	repo       repo.Repo
	sys        System
	key        *KeyManager
	cmkAuditor *auditor.Auditor
}

type (
	// OffboardingResult represents the result of a tenant offboarding attempt.
	OffboardingResult struct {
		// Status indicates the outcome of the offboarding process.
		Status OffboardingStatus
	}

	// OffboardingStatus represents the status of the tenant offboarding process.
	OffboardingStatus int
)

const (
	OffboardingProcessing OffboardingStatus = iota + 1
	OffboardingFailed
	OffboardingSuccess
)

func NewTenantManager(
	repo repo.Repo,
	sysManager System,
	keyManager *KeyManager,
	cmkAuditor *auditor.Auditor,
) *TenantManager {
	return &TenantManager{
		repo:       repo,
		sys:        sysManager,
		key:        keyManager,
		cmkAuditor: cmkAuditor,
	}
}

// OffboardTenant is a method to trigger the events to offboard a tenant
// - OffboardingProcessing: if any step is still in progress (retry later)
// - OffboardingFailed: if any step has failed permanently
// - OffboardingSuccess: if all steps completed successfully
// - error: if the offboarding process encounters an unexpected error, in which case it should be retried later
func (m *TenantManager) OffboardTenant(ctx context.Context) (OffboardingResult, error) {
	systemResult, err := m.unlinkAllSystems(ctx)
	if err != nil || systemResult.Status == OffboardingProcessing {
		return systemResult, err
	}

	keyResult, err := m.detatchAllKeys(ctx)
	if err != nil || keyResult.Status == OffboardingProcessing {
		return keyResult, err
	}

	return OffboardingResult{OffboardingSuccess}, nil
}

func (m *TenantManager) DeleteTenant(ctx context.Context) error {
	return m.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		tenantID, err := cmkcontext.ExtractTenantID(ctx)
		if err != nil {
			return err
		}

		_, err = r.Delete(ctx, &model.Tenant{ID: tenantID}, *repo.NewQuery())
		if err != nil {
			return err
		}

		err = r.OffboardTenant(ctx, tenantID)
		if err != nil {
			return err
		}

		err = m.cmkAuditor.SendCmkTenantDeleteAuditLog(ctx, tenantID)
		if err != nil {
			log.Error(ctx, "Failed to send delete tenant log", err)
		}

		return nil
	})
}

func (m *TenantManager) GetTenant(ctx context.Context) (*model.Tenant, error) {
	t, err := repo.GetTenant(ctx, m.repo)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (m *TenantManager) ListTenantInfo(
	ctx context.Context,
	issuerURL *string,
	skip int,
	top int,
) ([]*model.Tenant, int, error) {
	var tenants []*model.Tenant

	query := repo.NewQuery().SetLimit(top).SetOffset(skip)

	if issuerURL != nil {
		ck := repo.NewCompositeKey().Where(repo.IssuerURLField, issuerURL)
		query = query.Where(repo.NewCompositeKeyGroup(ck))
	}

	count, err := m.repo.List(ctx, model.Tenant{}, &tenants, *query)
	if err != nil {
		return nil, 0, ErrListTenants
	}

	return tenants, count, nil
}

func (m *TenantManager) CreateTenant(ctx context.Context, tenant *model.Tenant) error {
	err := validateSchema(tenant.SchemaName)
	if err != nil {
		return errs.Wrap(repo.ErrOnboardingTenant, err)
	}

	err = tenant.Validate()
	if err != nil {
		return errs.Wrap(ErrValidatingTenant, err)
	}

	err = m.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		err = r.Create(ctx, tenant)
		if err != nil {
			if errors.Is(err, repo.ErrUniqueConstraint) {
				err = errs.Wrap(ErrOnboardingInProgress, err)
			}

			return errs.Wrap(ErrCreatingTenant, err)
		}

		log.Info(ctx, "Tenant added to public schema")

		return r.Migrate(ctx, tenant.SchemaName)
	})

	return err
}

func (m *TenantManager) GetTenantByID(ctx context.Context, tenantID string) (*model.Tenant, error) {
	t, err := repo.GetTenantByID(ctx, m.repo, tenantID)
	if err != nil {
		return nil, err
	}

	return t, nil
}

// unlinkSystems triggers system delete events. On a successful created event
// the system status is changed to processing. It's considered a success if
// all systems are no longer connected or in processing
func (m *TenantManager) unlinkAllSystems(ctx context.Context) (OffboardingResult, error) {
	result := OffboardingResult{Status: OffboardingSuccess}
	toUnlinkCond := repo.NewCompositeKey().
		Where(repo.StatusField, cmkapi.SystemStatusCONNECTED).
		Where(repo.StatusField, cmkapi.SystemStatusPROCESSING, repo.NotEq)

	err := repo.ProcessInBatch(
		ctx,
		m.repo,
		repo.NewQuery().Where(repo.NewCompositeKeyGroup(toUnlinkCond)),
		repo.DefaultLimit,
		func(sys []*model.System) error {
			for _, s := range sys {
				err := m.sys.DeleteSystemLinkByID(ctx, s.ID)
				if err != nil {
					return err
				}
			}

			return nil
		},
	)
	if err != nil {
		return OffboardingResult{}, err
	}

	unlinkingCond := repo.NewCompositeKey().
		Where(repo.StatusField, cmkapi.SystemStatusCONNECTED).
		Where(repo.StatusField, cmkapi.SystemStatusPROCESSING)
	unlinkingCond.IsStrict = false

	count, err := m.repo.Count(
		ctx,
		&model.System{},
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(unlinkingCond)),
	)
	if err != nil {
		return OffboardingResult{}, err
	}

	if count > 0 {
		return OffboardingResult{Status: OffboardingProcessing}, nil
	}

	return result, nil
}

// detatchAllKeys triggers key detatch events. On a successful created event
// the key state is changed to detached. It's considered a success if
// all keys are no longer enabled or disabled
func (m *TenantManager) detatchAllKeys(ctx context.Context) (OffboardingResult, error) {
	result := OffboardingResult{Status: OffboardingSuccess}

	detatch := repo.NewCompositeKey().
		Where(repo.StateField, cmkapi.KeyStateENABLED).
		Where(repo.StateField, cmkapi.KeyStateDISABLED)
	detatch.IsStrict = false

	err := repo.ProcessInBatch(
		ctx,
		m.repo,
		repo.NewQuery().Where(repo.NewCompositeKeyGroup(detatch)),
		repo.DefaultLimit,
		func(keys []*model.Key) error {
			for _, k := range keys {
				err := m.key.Detatch(ctx, k)
				if err != nil {
					return err
				}
			}

			return nil
		},
	)
	if err != nil {
		return OffboardingResult{}, err
	}

	count, err := m.repo.Count(
		ctx,
		&model.Key{},
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(detatch)),
	)
	if err != nil {
		return OffboardingResult{}, err
	}

	if count > 0 {
		return OffboardingResult{Status: OffboardingProcessing}, nil
	}

	return result, nil
}

func validateSchema(schema string) error {
	err := namespace.Validate(schema)
	if err != nil {
		return errs.Wrap(ErrInvalidSchema, err)
	}

	if len(schema) < 3 || len(schema) > 63 {
		return ErrSchemaNameLength
	}

	return nil
}
