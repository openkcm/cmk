package tenant_manager_test

import (
	"context"
	"testing"
	"time"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	integrationutils "github.com/openkcm/cmk/test/integration_utils"
	"github.com/openkcm/cmk/utils/base62"
)

func TestRegistryTenantManagerIntegration(t *testing.T) {
	// given
	tenantClient := setupGRPC(t)
	ctx := t.Context()
	_, multitenancyDB := setupDB(t)

	t.Run("Should provision tenant in registry and cmk", func(t *testing.T) {
		// given
		tenantID := uuid.NewString()
		req := &tenantgrpc.RegisterTenantRequest{
			Name:      "SuccessFactor2",
			Id:        tenantID,
			Region:    "emea",
			OwnerId:   "owner123",
			OwnerType: "owner_type",
			Role:      tenantgrpc.Role_ROLE_LIVE,
		}

		// when
		t.Logf("Send 'RegisterTenant' request")

		resp, err := tenantClient.RegisterTenant(ctx, req)

		// then
		require.NoError(t, err)
		require.NotNil(t, resp)
		activeTenant := assertTenantExistsInRegistry(ctx, t, tenantClient, req.GetId())
		assertTenantExistsInCMK(ctx, t, multitenancyDB, req.GetId(), req.GetRegion())
		assertDefaultGroupsExistInRegistry(t, activeTenant, req.GetId())
	})
}

func setupGRPC(t *testing.T) tenantgrpc.ServiceClient {
	t.Helper()

	clientsFactory, err := clients.NewFactory(config.Services{Registry: testutils.TestRegistryConfig})
	require.NoError(t, err)
	t.Cleanup(func() {
		err = clientsFactory.Close()
		if err != nil {
			t.Logf("Failed to close clients factory: %v", err)
		}
	})

	return clientsFactory.RegistryService().Tenant()
}

func setupDB(t *testing.T) (*sql.ResourceRepository, *multitenancy.DB) {
	t.Helper()

	multitenancyDB, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		RequiresMultitenancyOrShared: false, // false until testcontainers for TM is prepared to allow custom cmk db
		Models:                       []driver.TenantTabler{&model.Tenant{}, &model.Group{}},
	})

	return sql.NewRepository(multitenancyDB), multitenancyDB
}

func assertTenantExistsInRegistry(
	ctx context.Context,
	t *testing.T,
	client tenantgrpc.ServiceClient,
	tenantID string,
) *tenantgrpc.Tenant {
	t.Helper()

	var tenant *tenantgrpc.Tenant

	require.Eventuallyf(t, func() bool {
		result, err := client.ListTenants(ctx, &tenantgrpc.ListTenantsRequest{Id: tenantID})
		if err != nil || len(result.GetTenants()) != 1 {
			return false
		}

		tenant = result.GetTenants()[0]
		t.Logf("Check tenant status: %s", tenant.GetStatus().String())

		return tenant.GetStatus() == tenantgrpc.Status_STATUS_ACTIVE
	}, 30*time.Second, 3*time.Second, "Tenant should become active")

	return tenant
}

func assertTenantExistsInCMK(
	ctx context.Context,
	t *testing.T,
	multitenancyDB *multitenancy.DB,
	tenantID, region string,
) {
	t.Helper()

	schemaName, err := base62.EncodeSchemaNameBase62(tenantID)
	require.NoError(t, err)
	integrationutils.TenantExists(t, multitenancyDB, schemaName, model.Group{}.TableName())
	integrationutils.CheckRegion(ctx, t, multitenancyDB, tenantID, region)
	integrationutils.GroupsExists(ctx, t, tenantID, multitenancyDB)
}

func assertDefaultGroupsExistInRegistry(t *testing.T, tenant *tenantgrpc.Tenant, tenantID string) {
	t.Helper()

	groups := tenant.GetUserGroups()
	t.Logf("Tenant groups: %v", groups)
	require.Len(t, groups, 2, "There should be two user groups")

	iamAdmin := model.NewIAMIdentifier(constants.TenantAdminGroup, tenantID)
	iamAuditor := model.NewIAMIdentifier(constants.TenantAuditorGroup, tenantID)

	assert.Contains(t, groups, iamAdmin, "Admin group should be present")
	assert.Contains(t, groups, iamAuditor, "Auditor group should be present")
}
