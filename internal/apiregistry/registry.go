package apiregistry

import (
	mappingapi "github.com/openkcm/cmk/internal/apiregistry/api/mapping"
	oidcmappingapi "github.com/openkcm/cmk/internal/apiregistry/api/oidcmapping"
	"github.com/openkcm/cmk/internal/apiregistry/api/system"
	"github.com/openkcm/cmk/internal/apiregistry/api/tenant"
	mappingwrapper "github.com/openkcm/cmk/internal/apiregistry/wrapper/mapping"
	oidcmappingwrapper "github.com/openkcm/cmk/internal/apiregistry/wrapper/oidcmapping"
	systemwrapper "github.com/openkcm/cmk/internal/apiregistry/wrapper/system"
	tenantwrapper "github.com/openkcm/cmk/internal/apiregistry/wrapper/tenant"
	"github.com/openkcm/cmk/internal/clients"
)

// Registry provides centralized access to all API registry clients.
// It serves as the main entry point for interacting with the CMK API services.
type Registry struct {
	tenant      tenantapi.RegistryTenant
	system      systemapi.RegistrySystem
	mapping     mappingapi.RegistryMapping
	oidcMapping oidcmappingapi.SessionManagerOIDCMapping
}

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

func NewRegistry(
	tenantClient tenantapi.RegistryTenant,
	systemClient systemapi.RegistrySystem,
	mappingClient mappingapi.RegistryMapping,
	oidcMappingClient oidcmappingapi.SessionManagerOIDCMapping,
) *Registry {
	return &Registry{
		tenant:      tenantClient,
		system:      systemClient,
		mapping:     mappingClient,
		oidcMapping: oidcMappingClient,
	}
}

func (r *Registry) Tenant() tenantapi.RegistryTenant {
	return r.tenant
}

func (r *Registry) System() systemapi.RegistrySystem {
	return r.system
}

func (r *Registry) Mapping() mappingapi.RegistryMapping {
	return r.mapping
}

func (r *Registry) OIDCMapping() oidcmappingapi.SessionManagerOIDCMapping {
	return r.oidcMapping
}
