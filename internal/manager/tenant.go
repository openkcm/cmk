package manager

import (
	"context"
	"errors"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/namespace"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

type Tenant interface {
	GetTenant(ctx context.Context) (*model.Tenant, error) // Get tenant from context
	ListTenantInfo(ctx context.Context, issuerURL *string, skip int, top int) ([]*model.Tenant, int, error)
	CreateTenant(ctx context.Context, tenant *model.Tenant) error
}

type TenantManager struct {
	repo repo.Repo
}

func NewTenantManager(
	repo repo.Repo,
) *TenantManager {
	return &TenantManager{
		repo: repo,
	}
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
