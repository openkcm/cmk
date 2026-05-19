package authz_policy_test

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
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	mockregistry "github.com/openkcm/cmk/internal/testutils/clients/registry"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// fakeTenantClient is a minimal tenantv1.ServiceClient that returns a fixed
// tenant name for GetTenant and panics on any unneeded method.
type fakeTenantClient struct {
	tenantv1.UnimplementedServiceServer

	name string
}

func (f *fakeTenantClient) GetTenant(_ context.Context, _ *tenantv1.GetTenantRequest, _ ...grpc.CallOption) (*tenantv1.GetTenantResponse, error) {
	return &tenantv1.GetTenantResponse{
		Tenant: &tenantv1.Tenant{Name: f.name},
	}, nil
}

func (f *fakeTenantClient) RegisterTenant(_ context.Context, _ *tenantv1.RegisterTenantRequest, _ ...grpc.CallOption) (*tenantv1.RegisterTenantResponse, error) {
	panic("not implemented")
}

func (f *fakeTenantClient) ListTenants(_ context.Context, _ *tenantv1.ListTenantsRequest, _ ...grpc.CallOption) (*tenantv1.ListTenantsResponse, error) {
	panic("not implemented")
}

func (f *fakeTenantClient) BlockTenant(_ context.Context, _ *tenantv1.BlockTenantRequest, _ ...grpc.CallOption) (*tenantv1.BlockTenantResponse, error) {
	panic("not implemented")
}

func (f *fakeTenantClient) UnblockTenant(_ context.Context, _ *tenantv1.UnblockTenantRequest, _ ...grpc.CallOption) (*tenantv1.UnblockTenantResponse, error) {
	panic("not implemented")
}

func (f *fakeTenantClient) TerminateTenant(_ context.Context, _ *tenantv1.TerminateTenantRequest, _ ...grpc.CallOption) (*tenantv1.TerminateTenantResponse, error) {
	panic("not implemented")
}

func (f *fakeTenantClient) SetTenantLabels(_ context.Context, _ *tenantv1.SetTenantLabelsRequest, _ ...grpc.CallOption) (*tenantv1.SetTenantLabelsResponse, error) {
	panic("not implemented")
}

func (f *fakeTenantClient) RemoveTenantLabels(_ context.Context, _ *tenantv1.RemoveTenantLabelsRequest, _ ...grpc.CallOption) (*tenantv1.RemoveTenantLabelsResponse, error) {
	panic("not implemented")
}

func (f *fakeTenantClient) SetTenantUserGroups(_ context.Context, _ *tenantv1.SetTenantUserGroupsRequest, _ ...grpc.CallOption) (*tenantv1.SetTenantUserGroupsResponse, error) {
	panic("not implemented")
}

// TestTenantNameRefresh_AuthzPolicy verifies that the InternalTaskTenantRefreshRole
// policy grants Update on Tenant, which is required by TenantNameRefresher.ProcessTask.
//
// A tenant with an empty Name is seeded so the task's TenantQuery filter matches
// and ProcessTask reaches the authz-guarded r.Patch call. The fake registry
// returns a valid tenant name so the Patch proceeds.
func TestTenantNameRefresh_AuthzPolicy(t *testing.T) {
	// Seed a tenant with an empty name so it is picked up by TenantQuery
	// (WHERE name = '').
	emptyNameTenant := testutils.NewTenant(func(t *model.Tenant) {
		t.Name = ""
	})

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	}, testutils.WithInitTenants(*emptyNameTenant))

	tenant := tenants[0]
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	ctx, err := cmkcontext.InjectInternalUserData(ctx, constants.InternalTaskTenantRefreshRole)
	assert.NoError(t, err)

	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(), r, &config.Config{})
	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	fakeClient := &fakeTenantClient{name: "my-tenant-name"}
	reg := mockregistry.NewMockService(nil, fakeClient, nil)

	refresher := tasks.NewTenantNameRefresher(authzRepo, reg)
	task := asynq.NewTask(config.TypeTenantRefreshName, nil)

	// The registry returns a name, so the Patch (authz-guarded) will be called.
	// InternalTaskTenantRefreshRole must permit Update on Tenant.
	t.Run("InternalTaskTenantRefreshRole allows Update on Tenant", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		err := refresher.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.NotContains(t, strings.ToLower(buf.String()), "error",
			"unexpected error log: %s", buf.String())
	})
}
