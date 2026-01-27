package authzmodel_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/authz"
	authzmodel "github.com/openkcm/cmk/internal/authz-model"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	repomock "github.com/openkcm/cmk/internal/repo/mock"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestAuthzManager_LoadEntitiesInAllowList(t *testing.T) {
	repo := repomock.NewInMemoryRepository()

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
	// This is necessary to simulate the environment where the Engine operates
	// Each tenantID will have its own set of groups with different roles
	// The Engine will then load these groups into its allowlist
	// and ensure that the roles are correctly assigned to the tenantID
	for _, ts := range tenants {
		ctx := testutils.CreateCtxWithTenant(ts.tenantID)
		err := repo.Create(
			ctx, &model.Tenant{
				TenantModel: multitenancy.TenantModel{
					DomainURL:  "",
					SchemaName: "",
				},
				ID:     ts.tenantID,
				Region: ts.tenantID,
				Status: "Test",
			},
		)
		assert.NoError(t, err, "Failed to create tenantID %s", ts.tenantID)

		for _, g := range ts.groups {
			err := repo.Create(ctx, g)
			assert.NoError(t, err, "Failed to create group for tenantID %s: %v", ts.tenantID, g.Name)
		}
	}

	cfg := &config.Config{}
	am := authzmodel.NewEngine(t.Context(), repo, cfg)

	expectedRoles := []constants.Role{constants.TenantAdminRole, constants.TenantAuditorRole, constants.KeyAdminRole}

	// Helper to check roles for a tenantID
	checkRoles := func(entities []authz.Entity, tenant string, expectedRoles []constants.Role) {
		roles := map[constants.Role]bool{}

		for _, e := range entities {
			if string(e.TenantID) == tenant {
				roles[e.Role] = true
			}
		}

		for _, role := range expectedRoles {
			assert.Truef(t, roles[role], "Role %s missing for tenantID %s", role, tenant)
		}
	}

	// Load and check for each tenantID
	for _, ts := range tenants {
		ctx := testutils.CreateCtxWithTenant(ts.tenantID)
		err := am.LoadAllowList(ctx, ts.tenantID)
		assert.NoError(t, err)
		checkRoles(am.AuthzHandler.Entities, ts.tenantID, expectedRoles)
	}

	// Check that both tenants' roles are present in the entities
	entities := am.AuthzHandler.Entities
	for _, ts := range tenants {
		checkRoles(entities, ts.tenantID, expectedRoles)
	}

	// Reload for tenant1 and check again
	ctx1 := testutils.CreateCtxWithTenant(tenants[0].tenantID)
	err := am.ReloadAllowList(ctx1)
	assert.NoError(t, err)
	checkRoles(am.AuthzHandler.Entities, tenants[0].tenantID, expectedRoles)

	err = am.LoadAllowList(ctx1, tenants[0].tenantID)
	assert.NoError(t, err)
}
