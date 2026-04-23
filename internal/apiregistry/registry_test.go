package apiregistry_test

import (
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/apiregistry"
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
