package apiregistry

import (
	"errors"
	"time"

	"github.com/openkcm/common-sdk/pkg/commongrpc"

	mappinggrpcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
	tenantgrpcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	oidcmappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"

	mappingapi "github.com/openkcm/cmk/internal/apiregistry/api/mapping"
	oidcmappingapi "github.com/openkcm/cmk/internal/apiregistry/api/oidcmapping"
	systemapi "github.com/openkcm/cmk/internal/apiregistry/api/system"
	tenantapi "github.com/openkcm/cmk/internal/apiregistry/api/tenant"
	mappingwrapper "github.com/openkcm/cmk/internal/apiregistry/wrapper/mapping"
	oidcmappingwrapper "github.com/openkcm/cmk/internal/apiregistry/wrapper/oidcmapping"
	systemwrapper "github.com/openkcm/cmk/internal/apiregistry/wrapper/system"
	tenantwrapper "github.com/openkcm/cmk/internal/apiregistry/wrapper/tenant"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/clients/registry/systems"
	"github.com/openkcm/cmk/internal/config"
)

const (
	DefaultThrottleInterval = 5 * time.Second
)

var (
	ErrNotRegistered = errors.New("not registered")
)

type Registry interface {
	System() (systemapi.RegistrySystem, error)
	Tenant() (tenantapi.RegistryTenant, error)
	Mapping() (mappingapi.RegistryMapping, error)
	OIDCMapping() (oidcmappingapi.SessionManagerOIDCMapping, error)
}

// Registry provides centralized access to all API registry clients.
// It serves as the main entry point for interacting with the CMK API services.
type registryStruct struct {
	tenant      tenantapi.RegistryTenant
	system      systemapi.RegistrySystem
	mapping     mappingapi.RegistryMapping
	oidcMapping oidcmappingapi.SessionManagerOIDCMapping
}

var _ Registry = (*registryStruct)(nil)

func NewDeprecated(clientFactory clients.Factory) Registry {
	registryService := clientFactory.Registry()
	sessionManagerService := clientFactory.SessionManager()

	return &registryStruct{
		tenant:      tenantwrapper.NewV1(registryService.Tenant()),
		system:      systemwrapper.NewV1(registryService.System()),
		mapping:     mappingwrapper.NewV1(registryService.Mapping()),
		oidcMapping: oidcmappingwrapper.NewV1(sessionManagerService.OIDCMapping()),
	}
}

func NewRegistryDeprecated(
	tenantClient tenantapi.RegistryTenant,
	systemClient systemapi.RegistrySystem,
	mappingClient mappingapi.RegistryMapping,
	oidcMappingClient oidcmappingapi.SessionManagerOIDCMapping,
) Registry {
	return &registryStruct{
		tenant:      tenantClient,
		system:      systemClient,
		mapping:     mappingClient,
		oidcMapping: oidcMappingClient,
	}
}

func New(svs *config.Services) (Registry, error) {
	var tenant tenantapi.RegistryTenant
	var system systemapi.RegistrySystem
	var mapping mappingapi.RegistryMapping
	var oidcMapping oidcmappingapi.SessionManagerOIDCMapping

	if svs.Registry.Enabled {
		registryConn, err := commongrpc.NewDynamicClientConn(svs.Registry, DefaultThrottleInterval)
		if err != nil {
			return nil, err
		}
		sysClient, err := systems.NewSystemsClient(registryConn)
		if err != nil {
			return nil, err
		}

		tenant = tenantwrapper.NewV1(tenantgrpcv1.NewServiceClient(registryConn))
		system = systemwrapper.NewV1(sysClient)
		mapping = mappingwrapper.NewV1(mappinggrpcv1.NewServiceClient(registryConn))
	}

	if svs.SessionManager.Enabled {
		smConn, err := commongrpc.NewDynamicClientConn(svs.SessionManager, DefaultThrottleInterval)
		if err != nil {
			return nil, err
		}
		oidcMapping = oidcmappingwrapper.NewV1(oidcmappinggrpc.NewServiceClient(smConn))
	}

	return &registryStruct{
		tenant:      tenant,
		system:      system,
		mapping:     mapping,
		oidcMapping: oidcMapping,
	}, nil
}

func (r *registryStruct) Tenant() (tenantapi.RegistryTenant, error) {
	if r.tenant == nil {
		return nil, ErrNotRegistered
	}

	return r.tenant, nil
}

func (r *registryStruct) System() (systemapi.RegistrySystem, error) {
	if r.system == nil {
		return nil, ErrNotRegistered
	}

	return r.system, nil
}

func (r *registryStruct) Mapping() (mappingapi.RegistryMapping, error) {
	if r.mapping == nil {
		return nil, ErrNotRegistered
	}

	return r.mapping, nil
}

func (r *registryStruct) OIDCMapping() (oidcmappingapi.SessionManagerOIDCMapping, error) {
	if r.oidcMapping == nil {
		return nil, ErrNotRegistered
	}

	return r.oidcMapping, nil
}
