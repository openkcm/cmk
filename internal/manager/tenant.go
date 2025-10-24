package manager

import (
	"context"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type Tenant interface {
	GetTenant(ctx context.Context) (*model.Tenant, error)
	ListTenantInfo(ctx context.Context, skip int, top int) ([]*model.Tenant, int, error)
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
	tenant, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return nil, err
	}

	t := &model.Tenant{}

	_, err = m.repo.First(ctx, t, *repo.NewQuery().
		Where(
			repo.NewCompositeKeyGroup(
				repo.NewCompositeKey().
					Where(repo.IDField, tenant),
			),
		),
	)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (m *TenantManager) ListTenantInfo(ctx context.Context, skip int, top int) ([]*model.Tenant, int, error) {
	var tentants []*model.Tenant

	count, err := m.repo.List(ctx, model.Tenant{}, &tentants, *repo.NewQuery().
		SetLimit(top).
		SetOffset(skip),
	)
	if err != nil {
		return nil, 0, ErrListTenants
	}

	return tentants, count, nil
}
