package tasks_test

import (
	"context"
	"testing"

	"github.com/zeebo/assert"
	"google.golang.org/grpc"

	tenantv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk/internal/async/tasks"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/clients/registry"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

const RefreshedName = "refreshed-name"

type MockTenantRegistry struct {
	tenantv1.UnimplementedServiceServer
}

func (m *MockTenantRegistry) RegisterTenant(ctx context.Context, in *tenantv1.RegisterTenantRequest, opts ...grpc.CallOption) (*tenantv1.RegisterTenantResponse, error) {
	panic("not implemented")
}

func (m *MockTenantRegistry) ListTenants(ctx context.Context, in *tenantv1.ListTenantsRequest, opts ...grpc.CallOption) (*tenantv1.ListTenantsResponse, error) {
	panic("not implemented")
}

func (m *MockTenantRegistry) GetTenant(ctx context.Context, in *tenantv1.GetTenantRequest, opts ...grpc.CallOption) (*tenantv1.GetTenantResponse, error) {
	return &tenantv1.GetTenantResponse{
		Tenant: &tenantv1.Tenant{
			Name: RefreshedName,
		},
	}, nil
}

func (m *MockTenantRegistry) BlockTenant(ctx context.Context, in *tenantv1.BlockTenantRequest, opts ...grpc.CallOption) (*tenantv1.BlockTenantResponse, error) {
	panic("not implemented")
}

func (m *MockTenantRegistry) UnblockTenant(ctx context.Context, in *tenantv1.UnblockTenantRequest, opts ...grpc.CallOption) (*tenantv1.UnblockTenantResponse, error) {
	panic("not implemented")
}

func (m *MockTenantRegistry) TerminateTenant(ctx context.Context, in *tenantv1.TerminateTenantRequest, opts ...grpc.CallOption) (*tenantv1.TerminateTenantResponse, error) {
	panic("not implemented")
}

func (m *MockTenantRegistry) SetTenantLabels(ctx context.Context, in *tenantv1.SetTenantLabelsRequest, opts ...grpc.CallOption) (*tenantv1.SetTenantLabelsResponse, error) {
	panic("not implemented")
}

func (m *MockTenantRegistry) RemoveTenantLabels(ctx context.Context, in *tenantv1.RemoveTenantLabelsRequest, opts ...grpc.CallOption) (*tenantv1.RemoveTenantLabelsResponse, error) {
	panic("not implemented")
}

func (m *MockTenantRegistry) SetTenantUserGroups(ctx context.Context, in *tenantv1.SetTenantUserGroupsRequest, opts ...grpc.CallOption) (*tenantv1.SetTenantUserGroupsResponse, error) {
	panic("not implemented")
}

func TestTenantNameRefresher(t *testing.T) {
	emptyNameTenant := testutils.NewTenant(func(t *model.Tenant) {
		t.Name = ""
	})
	namedTenant := testutils.NewTenant(func(t *model.Tenant) {})

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	}, testutils.WithInitTenants(*emptyNameTenant, *namedTenant))
	r := sql.NewRepository(db)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenants[0])

	registry := registry.NewMockService(nil, &MockTenantRegistry{}, nil)
	refresher := tasks.NewTenantNameRefresher(r, registry)

	t.Run("Should update tenant names if empty", func(t *testing.T) {
		err := refresher.ProcessTask(ctx, nil)
		assert.NoError(t, err)

		tenant := &model.Tenant{ID: emptyNameTenant.ID}
		_, err = r.First(ctx, tenant, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, RefreshedName, tenant.Name)

		tenant = &model.Tenant{ID: namedTenant.ID}
		_, err = r.First(ctx, tenant, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, namedTenant.Name, tenant.Name)
	})

	t.Run("Task type is correct", func(t *testing.T) {
		taskType := refresher.TaskType()
		assert.Equal(t, config.TypeTenantRefreshName, taskType)
	})

	t.Run("Should handle nil task parameter", func(t *testing.T) {
		err := refresher.ProcessTask(t.Context(), nil)
		assert.NoError(t, err)
	})
}
