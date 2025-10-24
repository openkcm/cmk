package clients

import (
	"errors"

	"github.com/openkcm/cmk-core/internal/clients/registry"
	"github.com/openkcm/cmk-core/internal/config"
)

type Factory struct {
	registryService *registry.Service

	cfg *config.Services
}

func NewFactory(svs config.Services) (*Factory, error) {
	factory := &Factory{
		cfg: &svs,
	}

	if svs.Registry != nil && svs.Registry.Enabled {
		registryService, err := registry.NewService(svs.Registry)
		if err != nil {
			return nil, err
		}

		factory.registryService = registryService
	}

	return factory, nil
}

func (f *Factory) RegistryService() *registry.Service {
	return f.registryService
}

func (f *Factory) Close() error {
	var errs []error

	if f.registryService != nil {
		err := f.registryService.Close()
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}
