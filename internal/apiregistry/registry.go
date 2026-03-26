package apiregistry

import (
	mappingwrapper "github.com/openkcm/cmk/internal/apiregistry/wrapper/mapping"
	oidcmappingwrapper "github.com/openkcm/cmk/internal/apiregistry/wrapper/oidcmapping"
	systemwrapper "github.com/openkcm/cmk/internal/apiregistry/wrapper/system"
	tenantwrapper "github.com/openkcm/cmk/internal/apiregistry/wrapper/tenant"
	"github.com/openkcm/cmk/internal/clients"
)

// Registry provides centralized access to all API registry clients.
// It serves as the main entry point for interacting with the CMK API services.
type Registry struct {
	tenant      *tenantwrapper.V1
	system      *systemwrapper.V1
	mapping     *mappingwrapper.V1
	oidcMapping *oidcmappingwrapper.V1
}

// New creates a new Registry from the client factory.
// It wraps the raw gRPC clients with our domain-specific wrappers.
func New(clientFactory clients.Factory) *Registry {
	registryService := clientFactory.Registry()
	sessionManagerService := clientFactory.SessionManager()

	return &Registry{
		tenant:      tenantwrapper.NewV1(registryService.Tenant()),
		system:      systemwrapper.NewV1(registryService.System()),
		mapping:     mappingwrapper.NewV1(registryService.Mapping()),
		oidcMapping: oidcmappingwrapper.NewV1(sessionManagerService.OIDCMapping()),
	}
}

// NewRegistry creates a new Registry with the provided API clients.
// Use this for custom initialization or testing.
func NewRegistry(
	tenantClient *tenantwrapper.V1,
	systemClient *systemwrapper.V1,
	mappingClient *mappingwrapper.V1,
	oidcMappingClient *oidcmappingwrapper.V1,
) *Registry {
	return &Registry{
		tenant:      tenantClient,
		system:      systemClient,
		mapping:     mappingClient,
		oidcMapping: oidcMappingClient,
	}
}

// Tenant returns the tenant API client.
func (r *Registry) Tenant() *tenantwrapper.V1 {
	return r.tenant
}

// System returns the system API client.
func (r *Registry) System() *systemwrapper.V1 {
	return r.system
}

// Mapping returns the mapping API client.
func (r *Registry) Mapping() *mappingwrapper.V1 {
	return r.mapping
}

// OIDCMapping returns the OIDC mapping API client.
func (r *Registry) OIDCMapping() *oidcmappingwrapper.V1 {
	return r.oidcMapping
}
