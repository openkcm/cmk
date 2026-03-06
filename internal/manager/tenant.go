package manager

import (
	"context"
	"errors"
	"strings"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/namespace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type Tenant interface {
	GetTenant(ctx context.Context) (*model.Tenant, error) // Get tenant from context
	ListTenantInfo(ctx context.Context, issuerURL *string, pagination repo.Pagination) ([]*model.Tenant, int, error)
	CreateTenant(ctx context.Context, tenant *model.Tenant) error
	OffboardTenant(ctx context.Context) (OffboardingResult, error)
	DeleteTenant(ctx context.Context) error
}

type TenantManager struct {
	repo       repo.Repo
	sys        System
	key        *KeyManager
	user       User
	cmkAuditor *auditor.Auditor
	migrator   db.Migrator
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
	user User,
	cmkAuditor *auditor.Auditor,
	migrator db.Migrator,
) *TenantManager {
	return &TenantManager{
		repo:       repo,
		sys:        sysManager,
		key:        keyManager,
		user:       user,
		cmkAuditor: cmkAuditor,
		migrator:   migrator,
	}
}

// OffboardTenant is a method to trigger the events to offboard a tenant
// - OffboardingProcessing: if any step is still in progress (retry later)
// - OffboardingFailed: if any step has failed permanently
// - OffboardingSuccess: if all steps completed successfully
// - error: if the offboarding process encounters an unexpected error, in which case it should be retried later
func (m *TenantManager) OffboardTenant(ctx context.Context) (OffboardingResult, error) {
	// Step 1: Unlink all (remaining) connected systems
	err := m.sendUnlinkForConnectedSystems(ctx)
	if err != nil {
		return OffboardingResult{}, err
	}

	// Check if all systems are unlinked. If not, return processing status to reconcile later.
	systemsUnlinked, err := m.checkAllSystemsUnlinked(ctx)
	if err != nil {
		return OffboardingResult{}, err
	}

	if !systemsUnlinked {
		return OffboardingResult{Status: OffboardingProcessing}, nil
	}

	// Step 2: Detach all (remaining) primary keys
	err = m.detachPrimaryKeys(ctx)
	if err != nil {
		return OffboardingResult{}, err
	}

	// Now wait for all primary keys to be at least in detaching state.
	// This does not wait for the event tasks to respond.
	keysProcessed, err := m.checkAllPrimaryKeysProcessed(ctx)
	if err != nil {
		return OffboardingResult{}, err
	}

	if !keysProcessed {
		return OffboardingResult{Status: OffboardingProcessing}, nil
	}

	// Step 3: Unmap all systems from tenant. Not yet need to wait for step 2 to complete
	canContinue, st := m.unmapAllSystemsFromRegistry(ctx)
	if !canContinue {
		return OffboardingResult{Status: st}, nil
	}

	// Step 4: Wait until all keys are detached. After this we can delete
	// the tenant data and the tenant itself in the next step.
	keysDetached, err := m.checkAllPrimaryKeysDetached(ctx)
	if err != nil {
		return OffboardingResult{}, err
	}

	if !keysDetached {
		return OffboardingResult{Status: OffboardingProcessing}, nil
	}

	return OffboardingResult{OffboardingSuccess}, nil
}

func (m *TenantManager) DeleteTenant(ctx context.Context) error {
	return m.repo.Transaction(ctx, func(ctx context.Context) error {
		tenantID, err := cmkcontext.ExtractTenantID(ctx)
		if err != nil {
			return err
		}

		_, err = m.repo.Delete(ctx, &model.Tenant{ID: tenantID}, *repo.NewQuery())
		if err != nil {
			return err
		}

		err = m.repo.OffboardTenant(ctx, tenantID)
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
	accessible, err := m.user.HasTenantAccess(ctx)
	if err != nil {
		return nil, err
	}

	if !accessible {
		return nil, ErrTenantNotAllowed
	}

	t, err := repo.GetTenant(ctx, m.repo)
	if err != nil {
		return nil, errs.Wrap(ErrGetTenantInfo, err)
	}

	return t, nil
}

func (m *TenantManager) ListTenantInfo(
	ctx context.Context,
	issuerURL *string,
	pagination repo.Pagination,
) ([]*model.Tenant, int, error) {
	query := repo.NewQuery()

	if issuerURL != nil {
		ck := repo.NewCompositeKey().Where(repo.IssuerURLField, issuerURL)
		query = query.Where(repo.NewCompositeKeyGroup(ck))
	}

	return repo.ListAndCount(ctx, m.repo, pagination, model.Tenant{}, query)
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

	err = m.repo.Transaction(ctx, func(ctx context.Context) error {
		err = m.repo.Create(ctx, tenant)
		if err != nil {
			if errors.Is(err, repo.ErrUniqueConstraint) {
				err = errs.Wrap(ErrOnboardingInProgress, err)
			}

			return errs.Wrap(ErrCreatingTenant, err)
		}

		return m.migrator.MigrateTenantToLatest(ctx, tenant)
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

func (m *TenantManager) sendUnlinkForConnectedSystems(ctx context.Context) error {
	// Select all systems with status connected and send unlink events for them.
	// This will trigger the unlinking process for each system.
	condition := repo.NewCompositeKey().Where(repo.StatusField, cmkapi.SystemStatusCONNECTED)

	return repo.ProcessInBatch(
		ctx,
		m.repo,
		repo.NewQuery().Where(repo.NewCompositeKeyGroup(condition)),
		repo.DefaultLimit,
		func(sys []*model.System) error {
			for _, s := range sys {
				err := m.sys.UnlinkSystemAction(ctx, s.ID, constants.SystemActionDecommission)
				if err != nil {
					return err
				}
			}

			return nil
		},
	)
}

func (m *TenantManager) checkAllSystemsUnlinked(ctx context.Context) (bool, error) {
	// Unlinked systems will be removed from key configuration
	condition := repo.NewCompositeKey().Where(repo.KeyConfigIDField, repo.NotNull)
	count, err := m.repo.Count(
		ctx,
		&model.System{},
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(condition)),
	)
	if err != nil {
		return false, err
	}

	return count == 0, nil
}

func (m *TenantManager) detachPrimaryKeys(ctx context.Context) error {
	// List all primary keys that are not yet detached and trigger detach events for them.
	query := repo.NewCompositeKey().
		Where(repo.IsPrimaryField, true).
		Where(repo.StateField, cmkapi.KeyStateDETACHING, repo.NotEq).
		Where(repo.StateField, cmkapi.KeyStateDETACHED, repo.NotEq)

	return repo.ProcessInBatch(
		ctx,
		m.repo,
		repo.NewQuery().Where(repo.NewCompositeKeyGroup(query)),
		repo.DefaultLimit,
		func(keys []*model.Key) error {
			for _, k := range keys {
				err := m.key.Detach(ctx, k)
				if err != nil {
					return err
				}
			}

			return nil
		},
	)
}

func (m *TenantManager) unmapAllSystemsFromRegistry(ctx context.Context) (bool, OffboardingStatus) {
	hasOffboardingFailed := false
	hasOffboardingProcessing := false

	_ = repo.ProcessInBatch(
		ctx,
		m.repo,
		repo.NewQuery(),
		repo.DefaultLimit,
		func(systems []*model.System) error {
			for _, s := range systems {
				err := m.sys.UnmapSystemFromRegistry(ctx, s)
				success, st := m.unmapSystemErrorCanContinue(ctx, err)
				if success {
					continue
				}

				// Don't fail the whole offboarding process if unmapping a system fails,
				// but continue with the next ones and return the overall status at the end.
				//nolint:staticcheck
				if st == OffboardingFailed {
					hasOffboardingFailed = true
				} else if st == OffboardingProcessing {
					hasOffboardingProcessing = true
				}
			}

			return nil
		},
	)

	// Returns OffboardingFailed if any of the unmapping operations has failed with a non-retryable error.
	// Otherwise, returns OffboardingProcessing if any of the unmapping operations has failed with a retryable error.
	switch {
	case hasOffboardingFailed:
		return false, OffboardingFailed
	case hasOffboardingProcessing:
		return false, OffboardingProcessing
	default:
		return true, 0
	}
}

// Returns true if system is unmapped successfully, system is already unmapped before, or if system is deleted.
// Otherwise, return OffboardingStatus to indicate if offboarding should continue or if it has failed.
// - OffboardingFailed: if the error is not retryable or invalid arguments are provided
// - OffboardingProcessing: if the error is retryable and offboarding should continue in next reconciliation loop
func (m *TenantManager) unmapSystemErrorCanContinue(ctx context.Context, err error) (bool, OffboardingStatus) {
	if err == nil {
		return true, 0
	}

	st, ok := status.FromError(err)
	if !ok {
		log.Error(ctx, "failed getting status from error when removing system mapping", err)
	}
	switch {
	case st.Code() == codes.FailedPrecondition && strings.Contains(st.Message(), "system is not linked to the tenant"):
		log.Info(ctx, "system is not linked to the tenant in registry, might have been already unlinked")
		return true, 0
	case st.Code() == codes.NotFound && strings.Contains(st.Message(), "system not found"):
		log.Info(ctx, "system mapping not found in registry, might have been already removed")
		return true, 0
	case st.Code() == codes.InvalidArgument:
		log.Error(ctx, "invalid argument error while unmapping system from tenant", err)
		return false, OffboardingFailed
	case err != nil:
		log.Error(ctx, "error while removing OIDC mapping", err)
		return false, OffboardingProcessing
	default:
		return true, 0
	}
}

func (m *TenantManager) checkAllPrimaryKeysProcessed(ctx context.Context) (bool, error) {
	query := repo.NewCompositeKey().
		Where(repo.IsPrimaryField, true).
		Where(repo.StateField, cmkapi.KeyStateDETACHING, repo.NotEq).
		Where(repo.StateField, cmkapi.KeyStateDETACHED, repo.NotEq)

	count, err := m.repo.Count(
		ctx,
		&model.Key{},
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(query)),
	)
	if err != nil {
		return false, err
	}

	return count == 0, nil
}

func (m *TenantManager) checkAllPrimaryKeysDetached(ctx context.Context) (bool, error) {
	query := repo.NewCompositeKey().
		Where(repo.IsPrimaryField, true).
		Where(repo.StateField, cmkapi.KeyStateDETACHED, repo.NotEq)

	count, err := m.repo.Count(
		ctx,
		&model.Key{},
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(query)),
	)
	if err != nil {
		return false, err
	}

	return count == 0, nil
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
