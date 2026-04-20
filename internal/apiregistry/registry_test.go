package apiregistry_test

import (
	"context"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/apiregistry"
	mappingapi "github.com/openkcm/cmk/internal/apiregistry/api/mapping"
	oidcmappingapi "github.com/openkcm/cmk/internal/apiregistry/api/oidcmapping"
	systemapi "github.com/openkcm/cmk/internal/apiregistry/api/system"
	tenantapi "github.com/openkcm/cmk/internal/apiregistry/api/tenant"
	"github.com/openkcm/cmk/internal/config"
)

// Helper function to initialize config.Services with different configurations
func initializeServices(registryEnabled bool, registryAddress string, sessionManagerEnabled bool, sessionManagerAddress string) *config.Services {
	return &config.Services{
		Registry: &commoncfg.GRPCClient{
			Enabled: registryEnabled,
			Address: registryAddress,
		},
		SessionManager: &commoncfg.GRPCClient{
			Enabled: sessionManagerEnabled,
			Address: sessionManagerAddress,
		},
	}
}

func TestNew_WithAllServicesEnabled(t *testing.T) {
	services := initializeServices(true, ":8080", true, ":8081")

	reg, err := apiregistry.New(services)
	assert.NoError(t, err)
	assert.NotNil(t, reg)

	_, err = reg.Tenant()
	assert.NoError(t, err)
	_, err = reg.System()
	assert.NoError(t, err)
	_, err = reg.Mapping()
	assert.NoError(t, err)
	_, err = reg.OIDCMapping()
	assert.NoError(t, err)
}

func TestNew_WithOnlyRegistryServiceEnabled(t *testing.T) {
	services := initializeServices(true, ":8080", false, "")

	reg, err := apiregistry.New(services)
	assert.NoError(t, err)
	assert.NotNil(t, reg)

	_, err = reg.Tenant()
	assert.NoError(t, err)
	_, err = reg.System()
	assert.NoError(t, err)
	_, err = reg.Mapping()
	assert.NoError(t, err)
	_, err = reg.OIDCMapping()
	assert.Error(t, err)
}

func TestNew_WithOnlySessionManagerServiceEnabled(t *testing.T) {
	services := initializeServices(false, "", true, ":8081")

	reg, err := apiregistry.New(services)
	assert.NoError(t, err)
	assert.NotNil(t, reg)

	_, err = reg.Tenant()
	assert.Error(t, err)
	_, err = reg.System()
	assert.Error(t, err)
	_, err = reg.Mapping()
	assert.Error(t, err)
	_, err = reg.OIDCMapping()
	assert.NoError(t, err)
}

func TestNew_WithInvalidRegistryEndpoint(t *testing.T) {
	services := initializeServices(true, ":///", false, "")

	svc, err := apiregistry.New(services)
	assert.NoError(t, err)

	tenant, err := svc.Tenant()
	assert.NoError(t, err)

	_, err = tenant.SetTenantLabels(t.Context(), &tenantapi.SetTenantLabelsRequest{
		ID:     "test-tenant",
		Labels: map[string]string{"env": "test"},
	})
	assert.Error(t, err)
}

func TestRegistryGetters_WithAllServicesDisabled(t *testing.T) {
	// Create a registry with all services disabled
	services := initializeServices(false, "", false, "")

	reg, err := apiregistry.New(services)
	assert.NoError(t, err)

	_, err = reg.Tenant()
	assert.Error(t, err)
	_, err = reg.System()
	assert.Error(t, err)
	_, err = reg.Mapping()
	assert.Error(t, err)
	_, err = reg.OIDCMapping()
	assert.Error(t, err)
}

// Mock implementations for testing

type mockTenantClient struct{}

func (m *mockTenantClient) RegisterTenant(ctx context.Context, req *tenantapi.RegisterTenantRequest) (*tenantapi.RegisterTenantResponse, error) {
	return &tenantapi.RegisterTenantResponse{ID: "test-id"}, nil
}

func (m *mockTenantClient) ListTenants(ctx context.Context, req *tenantapi.ListTenantsRequest) (*tenantapi.ListTenantsResponse, error) {
	return &tenantapi.ListTenantsResponse{}, nil
}

func (m *mockTenantClient) GetTenant(ctx context.Context, req *tenantapi.GetTenantRequest) (*tenantapi.GetTenantResponse, error) {
	return &tenantapi.GetTenantResponse{}, nil
}

func (m *mockTenantClient) BlockTenant(ctx context.Context, req *tenantapi.BlockTenantRequest) (*tenantapi.BlockTenantResponse, error) {
	return &tenantapi.BlockTenantResponse{Success: true}, nil
}

func (m *mockTenantClient) UnblockTenant(ctx context.Context, req *tenantapi.UnblockTenantRequest) (*tenantapi.UnblockTenantResponse, error) {
	return &tenantapi.UnblockTenantResponse{Success: true}, nil
}

func (m *mockTenantClient) TerminateTenant(ctx context.Context, req *tenantapi.TerminateTenantRequest) (*tenantapi.TerminateTenantResponse, error) {
	return &tenantapi.TerminateTenantResponse{Success: true}, nil
}

func (m *mockTenantClient) SetTenantLabels(ctx context.Context, req *tenantapi.SetTenantLabelsRequest) (*tenantapi.SetTenantLabelsResponse, error) {
	return &tenantapi.SetTenantLabelsResponse{Success: true}, nil
}

func (m *mockTenantClient) RemoveTenantLabels(ctx context.Context, req *tenantapi.RemoveTenantLabelsRequest) (*tenantapi.RemoveTenantLabelsResponse, error) {
	return &tenantapi.RemoveTenantLabelsResponse{Success: true}, nil
}

func (m *mockTenantClient) SetTenantUserGroups(ctx context.Context, req *tenantapi.SetTenantUserGroupsRequest) (*tenantapi.SetTenantUserGroupsResponse, error) {
	return &tenantapi.SetTenantUserGroupsResponse{Success: true}, nil
}

type mockSystemClient struct{}

func (m *mockSystemClient) RegisterSystem(ctx context.Context, req *systemapi.RegisterSystemRequest) (*systemapi.RegisterSystemResponse, error) {
	return &systemapi.RegisterSystemResponse{}, nil
}

func (m *mockSystemClient) ListSystems(ctx context.Context, req *systemapi.ListSystemsRequest) (*systemapi.ListSystemsResponse, error) {
	return &systemapi.ListSystemsResponse{}, nil
}

func (m *mockSystemClient) UpdateSystemL1KeyClaim(ctx context.Context, req *systemapi.UpdateSystemL1KeyClaimRequest) (*systemapi.UpdateSystemL1KeyClaimResponse, error) {
	return &systemapi.UpdateSystemL1KeyClaimResponse{Success: true}, nil
}

func (m *mockSystemClient) DeleteSystem(ctx context.Context, req *systemapi.DeleteSystemRequest) (*systemapi.DeleteSystemResponse, error) {
	return &systemapi.DeleteSystemResponse{Success: true}, nil
}

func (m *mockSystemClient) UpdateSystemStatus(ctx context.Context, req *systemapi.UpdateSystemStatusRequest) (*systemapi.UpdateSystemStatusResponse, error) {
	return &systemapi.UpdateSystemStatusResponse{Success: true}, nil
}

func (m *mockSystemClient) SetSystemLabels(ctx context.Context, req *systemapi.SetSystemLabelsRequest) (*systemapi.SetSystemLabelsResponse, error) {
	return &systemapi.SetSystemLabelsResponse{Success: true}, nil
}

func (m *mockSystemClient) RemoveSystemLabels(ctx context.Context, req *systemapi.RemoveSystemLabelsRequest) (*systemapi.RemoveSystemLabelsResponse, error) {
	return &systemapi.RemoveSystemLabelsResponse{Success: true}, nil
}

type mockMappingClient struct{}

func (m *mockMappingClient) MapSystemToTenant(ctx context.Context, req *mappingapi.MapSystemToTenantRequest) (*mappingapi.MapSystemToTenantResponse, error) {
	return &mappingapi.MapSystemToTenantResponse{Success: true}, nil
}

func (m *mockMappingClient) UnmapSystemFromTenant(ctx context.Context, req *mappingapi.UnmapSystemFromTenantRequest) (*mappingapi.UnmapSystemFromTenantResponse, error) {
	return &mappingapi.UnmapSystemFromTenantResponse{Success: true}, nil
}

func (m *mockMappingClient) Get(ctx context.Context, req *mappingapi.GetRequest) (*mappingapi.GetResponse, error) {
	return &mappingapi.GetResponse{TenantID: "test-tenant-id"}, nil
}

type mockOIDCMappingClient struct{}

func (m *mockOIDCMappingClient) ApplyOIDCMapping(ctx context.Context, req *oidcmappingapi.ApplyOIDCMappingRequest) (*oidcmappingapi.ApplyOIDCMappingResponse, error) {
	return &oidcmappingapi.ApplyOIDCMappingResponse{Success: true}, nil
}

func (m *mockOIDCMappingClient) RemoveOIDCMapping(ctx context.Context, req *oidcmappingapi.RemoveOIDCMappingRequest) (*oidcmappingapi.RemoveOIDCMappingResponse, error) {
	return &oidcmappingapi.RemoveOIDCMappingResponse{Success: true}, nil
}

func (m *mockOIDCMappingClient) BlockOIDCMapping(ctx context.Context, req *oidcmappingapi.BlockOIDCMappingRequest) (*oidcmappingapi.BlockOIDCMappingResponse, error) {
	return &oidcmappingapi.BlockOIDCMappingResponse{Success: true}, nil
}

func (m *mockOIDCMappingClient) UnblockOIDCMapping(ctx context.Context, req *oidcmappingapi.UnblockOIDCMappingRequest) (*oidcmappingapi.UnblockOIDCMappingResponse, error) {
	return &oidcmappingapi.UnblockOIDCMappingResponse{Success: true}, nil
}

// Tests for NewRegistryDeprecated

func TestNewRegistryDeprecated_WithAllClientsProvided(t *testing.T) {
	tenantClient := &mockTenantClient{}
	systemClient := &mockSystemClient{}
	mappingClient := &mockMappingClient{}
	oidcMappingClient := &mockOIDCMappingClient{}

	reg := apiregistry.NewRegistryDeprecated(
		tenantClient,
		systemClient,
		mappingClient,
		oidcMappingClient,
	)

	assert.NotNil(t, reg)

	// Test that all getters return the correct clients without errors
	tenant, err := reg.Tenant()
	assert.NoError(t, err)
	assert.NotNil(t, tenant)

	system, err := reg.System()
	assert.NoError(t, err)
	assert.NotNil(t, system)

	mapping, err := reg.Mapping()
	assert.NoError(t, err)
	assert.NotNil(t, mapping)

	oidcMapping, err := reg.OIDCMapping()
	assert.NoError(t, err)
	assert.NotNil(t, oidcMapping)
}

func TestNewRegistryDeprecated_WithNilClients(t *testing.T) {
	// Create registry with all nil clients
	reg := apiregistry.NewRegistryDeprecated(nil, nil, nil, nil)

	assert.NotNil(t, reg)

	// All getters should return ErrNotRegistered
	_, err := reg.Tenant()
	assert.ErrorIs(t, err, apiregistry.ErrNotRegistered)

	_, err = reg.System()
	assert.ErrorIs(t, err, apiregistry.ErrNotRegistered)

	_, err = reg.Mapping()
	assert.ErrorIs(t, err, apiregistry.ErrNotRegistered)

	_, err = reg.OIDCMapping()
	assert.ErrorIs(t, err, apiregistry.ErrNotRegistered)
}

func TestNewRegistryDeprecated_WithPartialClients(t *testing.T) {
	tests := []struct {
		name             string
		tenant           tenantapi.RegistryTenant
		system           systemapi.RegistrySystem
		mapping          mappingapi.RegistryMapping
		oidcMapping      oidcmappingapi.SessionManagerOIDCMapping
		expectTenantErr  bool
		expectSystemErr  bool
		expectMappingErr bool
		expectOIDCErr    bool
	}{
		{
			name:             "only tenant client",
			tenant:           &mockTenantClient{},
			expectTenantErr:  false,
			expectSystemErr:  true,
			expectMappingErr: true,
			expectOIDCErr:    true,
		},
		{
			name:             "only system client",
			system:           &mockSystemClient{},
			expectTenantErr:  true,
			expectSystemErr:  false,
			expectMappingErr: true,
			expectOIDCErr:    true,
		},
		{
			name:             "only mapping client",
			mapping:          &mockMappingClient{},
			expectTenantErr:  true,
			expectSystemErr:  true,
			expectMappingErr: false,
			expectOIDCErr:    true,
		},
		{
			name:             "only oidc mapping client",
			oidcMapping:      &mockOIDCMappingClient{},
			expectTenantErr:  true,
			expectSystemErr:  true,
			expectMappingErr: true,
			expectOIDCErr:    false,
		},
		{
			name:             "tenant and system clients",
			tenant:           &mockTenantClient{},
			system:           &mockSystemClient{},
			expectTenantErr:  false,
			expectSystemErr:  false,
			expectMappingErr: true,
			expectOIDCErr:    true,
		},
		{
			name:             "mapping and oidc clients",
			mapping:          &mockMappingClient{},
			oidcMapping:      &mockOIDCMappingClient{},
			expectTenantErr:  true,
			expectSystemErr:  true,
			expectMappingErr: false,
			expectOIDCErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := apiregistry.NewRegistryDeprecated(tt.tenant, tt.system, tt.mapping, tt.oidcMapping)

			assert.NotNil(t, reg)

			checkClientError(t, "Tenant", func() error { _, err := reg.Tenant(); return err }, tt.expectTenantErr)
			checkClientError(t, "System", func() error { _, err := reg.System(); return err }, tt.expectSystemErr)
			checkClientError(t, "Mapping", func() error { _, err := reg.Mapping(); return err }, tt.expectMappingErr)
			checkClientError(t, "OIDCMapping", func() error { _, err := reg.OIDCMapping(); return err }, tt.expectOIDCErr)
		})
	}
}

func checkClientError(t *testing.T, _ string, getter func() error, expectErr bool) {
	t.Helper()
	err := getter()
	if expectErr {
		assert.Error(t, err)
		assert.ErrorIs(t, err, apiregistry.ErrNotRegistered)
	} else {
		assert.NoError(t, err)
	}
}

func TestNewRegistryDeprecated_ClientFunctionality(t *testing.T) {
	// Test that the clients can actually be used
	tenantClient := &mockTenantClient{}
	systemClient := &mockSystemClient{}
	mappingClient := &mockMappingClient{}
	oidcMappingClient := &mockOIDCMappingClient{}

	reg := apiregistry.NewRegistryDeprecated(
		tenantClient,
		systemClient,
		mappingClient,
		oidcMappingClient,
	)

	ctx := context.Background()

	t.Run("tenant client", func(t *testing.T) {
		testTenantClient(t, ctx, reg)
	})

	t.Run("system client", func(t *testing.T) {
		testSystemClient(t, ctx, reg)
	})

	t.Run("mapping client", func(t *testing.T) {
		testMappingClient(t, ctx, reg)
	})

	t.Run("oidc mapping client", func(t *testing.T) {
		testOIDCMappingClient(t, ctx, reg)
	})
}

func testTenantClient(t *testing.T, ctx context.Context, reg apiregistry.Registry) {
	t.Helper()
	tenant, err := reg.Tenant()
	assert.NoError(t, err)

	resp, err := tenant.RegisterTenant(ctx, &tenantapi.RegisterTenantRequest{
		Name: "test-tenant",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "test-id", resp.ID)
}

func testSystemClient(t *testing.T, ctx context.Context, reg apiregistry.Registry) {
	t.Helper()
	system, err := reg.System()
	assert.NoError(t, err)

	sysResp, err := system.RegisterSystem(ctx, &systemapi.RegisterSystemRequest{
		Region:     "us-east-1",
		ExternalID: "test-system",
	})
	assert.NoError(t, err)
	assert.NotNil(t, sysResp)
}

func testMappingClient(t *testing.T, ctx context.Context, reg apiregistry.Registry) {
	t.Helper()
	mapping, err := reg.Mapping()
	assert.NoError(t, err)

	mapResp, err := mapping.MapSystemToTenant(ctx, &mappingapi.MapSystemToTenantRequest{
		TenantID:   "test-tenant",
		ExternalID: "test-system",
		Type:       "KEYSTORE",
	})
	assert.NoError(t, err)
	assert.NotNil(t, mapResp)
	assert.True(t, mapResp.Success)
}

func testOIDCMappingClient(t *testing.T, ctx context.Context, reg apiregistry.Registry) {
	t.Helper()
	oidcMapping, err := reg.OIDCMapping()
	assert.NoError(t, err)

	oidcResp, err := oidcMapping.ApplyOIDCMapping(ctx, &oidcmappingapi.ApplyOIDCMappingRequest{
		TenantID: "test-tenant",
		Issuer:   "https://example.com",
	})
	assert.NoError(t, err)
	assert.NotNil(t, oidcResp)
	assert.True(t, oidcResp.Success)
}
