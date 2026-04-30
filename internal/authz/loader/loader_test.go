package authz_loader_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	repomock "github.com/openkcm/cmk/internal/repo/mock"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestAuthzManager_LoadEntitiesInAllowList(t *testing.T) {
	r := repomock.NewInMemoryRepository()

	// Setup tenants and groups
	type tenantSetup struct {
		tenantID string
		groups   []*model.Group
	}

	tenants := []tenantSetup{
		{
			tenantID: "tenant1",
			groups: []*model.Group{
				{ID: uuid.New(), Name: "group1a", Role: constants.TenantAdminRole},
				{ID: uuid.New(), Name: "group1b", Role: constants.TenantAuditorRole},
				{ID: uuid.New(), Name: "group1c", Role: constants.KeyAdminRole},
				{ID: uuid.New(), Name: "group1d", Role: constants.KeyAdminRole},
			},
		},
		{
			tenantID: "tenant2",
			groups: []*model.Group{
				{ID: uuid.New(), Name: "group2a", Role: constants.TenantAdminRole},
				{ID: uuid.New(), Name: "group2b", Role: constants.TenantAuditorRole},
				{ID: uuid.New(), Name: "group2c", Role: constants.KeyAdminRole},
			},
		},
	}

	// Insert groups for each tenantID into the repository
	// and create the tenants in the repository
	// This is necessary to simulate the environment where the Loader operates
	// Each tenantID will have its own set of groups with different roles
	// The Loader will then load these groups into its allowlist
	// and ensure that the roles are correctly assigned to the tenantID
	for _, ts := range tenants {
		ctx := testutils.CreateCtxWithTenant(ts.tenantID)
		err := r.Create(
			ctx, &model.Tenant{
				TenantModel: multitenancy.TenantModel{
					DomainURL:  "",
					SchemaName: "",
				},
				ID:     ts.tenantID,
				Status: "Test",
			},
		)
		assert.NoError(t, err, "Failed to create tenantID %s", ts.tenantID)

		for _, g := range ts.groups {
			err := r.Create(ctx, g)
			assert.NoError(t, err, "Failed to create group for tenantID %s: %v", ts.tenantID, g.Name)
		}
	}

	cfg := &config.Config{}
	am := authz_loader.NewAPIAuthzLoader(t.Context(), r, cfg)

	numKeysPerTenant := 24

	// Load and check for each tenantID
	for tIndex, ts := range tenants {
		ctx := testutils.CreateCtxWithTenant(ts.tenantID)
		err := am.LoadAllowList(ctx)
		assert.NoError(t, err)
		assert.Len(t, am.AuthzHandler.BusinessUserAuthzData.AuthzKeys, (tIndex+1)*numKeysPerTenant)
	}

	// Reload for tenant1 and check again
	ctx1 := testutils.CreateCtxWithTenant(tenants[0].tenantID)
	err := am.ReloadAllowList(ctx1)
	assert.NoError(t, err)
	assert.Len(t, am.AuthzHandler.BusinessUserAuthzData.AuthzKeys, len(tenants)*numKeysPerTenant)

	err = am.LoadAllowList(ctx1)
	assert.NoError(t, err)
	assert.Len(t, am.AuthzHandler.BusinessUserAuthzData.AuthzKeys, len(tenants)*numKeysPerTenant)
}
