package tasks_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	tenantv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	tasks "github.com/openkcm/cmk/internal/async/tasks/tenant"
	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
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
	authzLoader *authz_loader.AuthzLoader[authz.RepoResourceTypeName,
		authz.RepoAction]
}

func (m *MockTenantRegistry) RegisterTenant(ctx context.Context, in *tenantv1.RegisterTenantRequest, opts ...grpc.CallOption) (*tenantv1.RegisterTenantResponse, error) {
	panic("not implemented")
}

func (m *MockTenantRegistry) ListTenants(ctx context.Context, in *tenantv1.ListTenantsRequest, opts ...grpc.CallOption) (*tenantv1.ListTenantsResponse, error) {
	panic("not implemented")
}

func (m *MockTenantRegistry) GetTenant(ctx context.Context, in *tenantv1.GetTenantRequest, opts ...grpc.CallOption) (*tenantv1.GetTenantResponse, error) {
	if m.authzLoader != nil {
		//We test for unauthz in this case
		m.authzLoader.LoadAllowList(ctx)
		_, err := authz.CheckAuthz(ctx, m.authzLoader.AuthzHandler,
			authz.RepoResourceTypeCertificate, authz.RepoActionDelete)
		return nil, err

	}
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

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(),
		r, &config.Config{})

	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	ctx := cmkcontext.CreateTenantContext(t.Context(), tenants[0])

	reg := registry.NewMockService(nil, &MockTenantRegistry{}, nil)
	refresher := tasks.NewTenantNameRefresher(authzRepo, reg)

	task := asynq.NewTask(config.TypeTenantRefreshName, nil)

	t.Run("Should update tenant names if empty", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		err := refresher.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.NotContains(t, strings.ToLower(buf.String()), "error")

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

	t.Run("Should have default tenant query", func(t *testing.T) {
		query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(repo.NewCompositeKey().Where(repo.Name, repo.Empty)))
		assert.Equal(t, query, refresher.TenantQuery())
	})

	t.Run("Should error if no tenantID", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		err := refresher.ProcessTask(t.Context(), task)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Error during tenant name refresh batch processing")
	})

	t.Run("Should log on unauthorized processing", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		mock := &MockTenantRegistry{authzLoader: authzRepoLoader}
		authzReg := registry.NewMockService(nil, mock, nil)
		authzRefresher := tasks.NewTenantNameRefresher(authzRepo, authzReg)
		err := authzRefresher.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Error during tenant name refresh batch processing")
		assert.Contains(t, buf.String(), "authorization decision error")
	})
}
