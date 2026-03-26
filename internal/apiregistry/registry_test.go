package apiregistry_test

import (
	"context"
	"testing"

	mappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	oidcmappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"

	"github.com/openkcm/cmk/internal/apiregistry"
	mappingwrapper "github.com/openkcm/cmk/internal/apiregistry/wrapper/mapping"
	oidcmappingwrapper "github.com/openkcm/cmk/internal/apiregistry/wrapper/oidcmapping"
	systemwrapper "github.com/openkcm/cmk/internal/apiregistry/wrapper/system"
	tenantwrapper "github.com/openkcm/cmk/internal/apiregistry/wrapper/tenant"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/clients/registry"
	"github.com/openkcm/cmk/internal/clients/registry/systems"
	sessionmanager "github.com/openkcm/cmk/internal/clients/session-manager"
	"github.com/openkcm/cmk/internal/model"
)

// mockRegistryService implements registry.Service for testing
type mockRegistryService struct {
	tenantClient  tenantgrpc.ServiceClient
	systemClient  systems.ServiceClient
	mappingClient mappinggrpc.ServiceClient
}

func (m *mockRegistryService) System() systems.ServiceClient {
	return m.systemClient
}

func (m *mockRegistryService) Tenant() tenantgrpc.ServiceClient {
	return m.tenantClient
}

func (m *mockRegistryService) Mapping() mappinggrpc.ServiceClient {
	return m.mappingClient
}

func (m *mockRegistryService) Close() error {
	return nil
}

// mockSessionManagerService implements sessionmanager.Service for testing
type mockSessionManagerService struct {
	oidcClient oidcmappinggrpc.ServiceClient
}

func (m *mockSessionManagerService) OIDCMapping() oidcmappinggrpc.ServiceClient {
	return m.oidcClient
}

func (m *mockSessionManagerService) Close() error {
	return nil
}

// mockClientFactory implements clients.Factory for testing
type mockClientFactory struct {
	registryService       registry.Service
	sessionManagerService sessionmanager.Service
}

func (m *mockClientFactory) Registry() registry.Service {
	return m.registryService
}

func (m *mockClientFactory) SessionManager() sessionmanager.Service {
	return m.sessionManagerService
}

func (m *mockClientFactory) Close() error {
	return nil
}

// mockSystemClient implements systems.ServiceClient for testing
type mockSystemClient struct {
	systemgrpc.ServiceClient
}

func (m *mockSystemClient) GetSystemsWithFilter(ctx context.Context, filter systems.SystemFilter) ([]*model.System, error) {
	return nil, nil
}

func (m *mockSystemClient) ExtendedUpdateSystemL1KeyClaim(ctx context.Context, filter systems.SystemFilter, l1KeyClaim bool) error {
	return nil
}

func TestNew(t *testing.T) {
	mockFactory := &mockClientFactory{
		registryService: &mockRegistryService{
			tenantClient:  &mockTenantClient{},
			systemClient:  &mockSystemClient{},
			mappingClient: &mockMappingClient{},
		},
		sessionManagerService: &mockSessionManagerService{
			oidcClient: &mockOIDCMappingClient{},
		},
	}

	reg := apiregistry.New(mockFactory)

	if reg == nil {
		t.Fatal("expected non-nil Registry instance")
	}

	if reg.Tenant() == nil {
		t.Error("expected non-nil tenant client")
	}

	if reg.System() == nil {
		t.Error("expected non-nil system client")
	}

	if reg.Mapping() == nil {
		t.Error("expected non-nil mapping client")
	}

	if reg.OIDCMapping() == nil {
		t.Error("expected non-nil OIDC mapping client")
	}
}

func TestNewRegistry(t *testing.T) {
	tenantClient := tenantwrapper.NewV1(&mockTenantClient{})
	systemClient := systemwrapper.NewV1(&mockSystemClient{})
	mappingClient := mappingwrapper.NewV1(&mockMappingClient{})
	oidcMappingClient := oidcmappingwrapper.NewV1(&mockOIDCMappingClient{})

	reg := apiregistry.NewRegistry(tenantClient, systemClient, mappingClient, oidcMappingClient)

	if reg == nil {
		t.Fatal("expected non-nil Registry instance")
	}

	if reg.Tenant() != tenantClient {
		t.Error("expected tenant client to match")
	}

	if reg.System() != systemClient {
		t.Error("expected system client to match")
	}

	if reg.Mapping() != mappingClient {
		t.Error("expected mapping client to match")
	}

	if reg.OIDCMapping() != oidcMappingClient {
		t.Error("expected OIDC mapping client to match")
	}
}

func TestRegistryGetters(t *testing.T) {
	tenantClient := tenantwrapper.NewV1(&mockTenantClient{})
	systemClient := systemwrapper.NewV1(&mockSystemClient{})
	mappingClient := mappingwrapper.NewV1(&mockMappingClient{})
	oidcMappingClient := oidcmappingwrapper.NewV1(&mockOIDCMappingClient{})

	reg := apiregistry.NewRegistry(tenantClient, systemClient, mappingClient, oidcMappingClient)

	t.Run("Tenant", func(t *testing.T) {
		if got := reg.Tenant(); got != tenantClient {
			t.Error("Tenant() returned unexpected client")
		}
	})

	t.Run("System", func(t *testing.T) {
		if got := reg.System(); got != systemClient {
			t.Error("System() returned unexpected client")
		}
	})

	t.Run("Mapping", func(t *testing.T) {
		if got := reg.Mapping(); got != mappingClient {
			t.Error("Mapping() returned unexpected client")
		}
	})

	t.Run("OIDCMapping", func(t *testing.T) {
		if got := reg.OIDCMapping(); got != oidcMappingClient {
			t.Error("OIDCMapping() returned unexpected client")
		}
	})
}

type mockTenantClient struct {
	tenantgrpc.ServiceClient
}

type mockMappingClient struct {
	mappinggrpc.ServiceClient
}

type mockOIDCMappingClient struct {
	oidcmappinggrpc.ServiceClient
}

var (
	_ tenantgrpc.ServiceClient      = (*mockTenantClient)(nil)
	_ mappinggrpc.ServiceClient     = (*mockMappingClient)(nil)
	_ oidcmappinggrpc.ServiceClient = (*mockOIDCMappingClient)(nil)
	_ clients.Factory               = (*mockClientFactory)(nil)
	_ registry.Service              = (*mockRegistryService)(nil)
	_ sessionmanager.Service        = (*mockSessionManagerService)(nil)
)
