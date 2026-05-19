package operator_test

// TestTenantProvisioning_AuthzPolicy verifies that InternalTenantProvisioningRole
// grants all the repo permissions the operator handlers require, without any
// manager being mocked.
//
// The three handlers tested correspond to the three AMQP task types the operator
// handles and that touch the database:
//
//   - handleCreateTenant  → probe.Check (First on Group) + CreateTenant (Create on
//     Tenant) + CreateDefaultGroups (Create on Group)
//   - handleApplyTenantAuth → applyOIDC (Update/Patch on Tenant)
//   - handleTerminateTenant → OffboardTenant: Count+List on System, Count+List on
//     Key (ProcessInBatch exit-early path with empty DB)
//     + DeleteTenant (Delete on Tenant)
//
// Each sub-test drives the handler directly via orbital.ExecuteHandler (no AMQP).
// Data is kept minimal: empty DB for terminate/create, one seeded tenant for
// applyOIDC.

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/respondertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	authgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"
	mappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	oidcmappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"

	"github.com/openkcm/cmk/internal/clients/registry/tenants"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/operator"
	"github.com/openkcm/cmk/internal/testutils"
	mockClient "github.com/openkcm/cmk/internal/testutils/clients"
	"github.com/openkcm/cmk/internal/testutils/clients/registry"
	sessionmanager "github.com/openkcm/cmk/internal/testutils/clients/session-manager"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
)

func TestTenantProvisioning_AuthzPolicy(t *testing.T) {
	ctx := createContext(t)

	// Shared test DB: CreateDatabase=true so that CreateTenant (which runs
	// MigrateTenantToLatest) can create a new schema. Two tenants are seeded:
	// one for handleApplyTenantAuth (which only updates) and one for
	// handleTerminateTenant (which deletes its tenant row).
	multitenancyDB, tenantList, cfgDB := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	}, testutils.WithGenerateTenants(2))

	ps := testutils.NewTestPlugins(testplugins.WithIdentityManagement(testplugins.NewTestIdentityManagement()))
	cfg := &config.Config{
		Database: cfgDB,
	}

	tenantManager, groupManager, authzRepo := createManagers(t, multitenancyDB, cfg, ps)

	// Fake session manager — used by handleApplyTenantAuth and handleTerminateTenant.
	fakeSessionManager := sessionmanager.NewFakeSessionManagerClient()
	fakeSessionManager.MockApplyOIDCMapping = func(
		_ context.Context,
		_ *oidcmappinggrpc.ApplyOIDCMappingRequest,
	) (*oidcmappinggrpc.ApplyOIDCMappingResponse, error) {
		return &oidcmappinggrpc.ApplyOIDCMappingResponse{Success: true}, nil
	}
	fakeSessionManager.MockRemoveOIDCMapping = func(
		_ context.Context,
		_ *oidcmappinggrpc.RemoveOIDCMappingRequest,
	) (*oidcmappinggrpc.RemoveOIDCMappingResponse, error) {
		return &oidcmappinggrpc.RemoveOIDCMappingResponse{Success: true}, nil
	}

	_, grpcConn := testutils.NewGRPCSuite(t, func(s *grpc.Server) {
		tenantgrpc.RegisterServiceServer(s, tenants.NewFakeTenantService())
	})
	clientFactory := mockClient.NewMockFactory(
		registry.NewMockService(nil, tenantgrpc.NewServiceClient(grpcConn), mappingv1.NewServiceClient(grpcConn)),
		sessionmanager.NewMockService(fakeSessionManager),
	)

	// respondertest.NewResponder() satisfies orbital.Responder and is
	// used only to satisfy the NewTenantOperator nil check; all three
	// sub-tests invoke handlers directly via orbital.ExecuteHandler.
	responder := respondertest.NewResponder()
	op, err := operator.NewTenantOperator(
		multitenancyDB,
		cfg,
		orbital.TargetOperator{Client: responder},
		clientFactory,
		tenantManager,
		groupManager,
		authzRepo,
	)
	require.NoError(t, err)

	// -----------------------------------------------------------------------
	// handleCreateTenant
	// -----------------------------------------------------------------------
	t.Run("InternalTenantProvisioningRole allows First+Create on Group and Create on Tenant", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		tenantID := uuid.NewString()
		data, err := createValidTenantData(tenantID, "us-east-1", "authz-test-tenant")
		require.NoError(t, err)

		req := buildRequest(uuid.New(), tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String(), data)
		resp := orbital.ExecuteHandler(ctx, op.HandleCreateTenant, req)

		// First invocation reaches probe.Check (First on Group) then
		// CreateTenant (Create on Tenant) and CreateDefaultGroups (Create on
		// Group) then returns PROCESSING to wait for registry confirmation.
		assert.Empty(t, resp.ErrorMessage,
			"unexpected error — possible authz denial: %s", buf.String())
		assert.NotContains(t, buf.String(), `"allowed":false`,
			"authz denial in log — policy is missing a required permission: %s", buf.String())
	})

	// -----------------------------------------------------------------------
	// handleApplyTenantAuth
	// -----------------------------------------------------------------------
	t.Run("InternalTenantProvisioningRole allows Update on Tenant", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		// Use a tenant that already exists in the DB so the Patch succeeds.
		tenantID := tenantList[0]

		authProto := &authgrpc.Auth{
			TenantId: tenantID,
			Properties: map[string]string{
				"issuer": "https://issuer.example.com",
			},
		}
		data, err := proto.Marshal(authProto)
		require.NoError(t, err)

		req := buildRequest(uuid.New(), authgrpc.AuthAction_AUTH_ACTION_APPLY_AUTH.String(), data)
		resp := orbital.ExecuteHandler(ctx, op.HandleApplyTenantAuth, req)

		assert.Empty(t, resp.ErrorMessage,
			"unexpected error — possible authz denial: %s", buf.String())
		assert.NotContains(t, buf.String(), `"allowed":false`,
			"authz denial in log — policy is missing a required permission: %s", buf.String())
	})

	// -----------------------------------------------------------------------
	// handleTerminateTenant
	// -----------------------------------------------------------------------
	t.Run("InternalTenantProvisioningRole allows Count+List on System and Key, Delete on Tenant", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		// Use a second tenant so it is not already deleted by a previous sub-test.
		tenantID := tenantList[1]

		tenant := &tenantgrpc.Tenant{Id: tenantID}
		data, err := proto.Marshal(tenant)
		require.NoError(t, err)

		req := buildRequest(uuid.New(), tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String(), data)
		resp := orbital.ExecuteHandler(ctx, op.HandleTerminateTenant, req)

		// No systems or primary keys are seeded → ProcessInBatch exits early on
		// System and Key → OffboardTenant returns OffboardingSuccess →
		// DeleteTenant deletes the tenant → handler returns DONE.
		assert.Empty(t, resp.ErrorMessage,
			"unexpected error — possible authz denial: %s", buf.String())
		assert.NotContains(t, buf.String(), `"allowed":false`,
			"authz denial in log — policy is missing a required permission: %s", buf.String())
	})
}
