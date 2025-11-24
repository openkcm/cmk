package clients

import (
	"errors"

	"github.com/openkcm/cmk/internal/clients/registry"
	sessionmanager "github.com/openkcm/cmk/internal/clients/session-manager"
	"github.com/openkcm/cmk/internal/config"
)

type Factory struct {
	registryService *registry.Service
	sessionManager  *sessionmanager.Client

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

	if svs.SessionManager != nil && svs.SessionManager.Enabled {
		sessionManager, err := sessionmanager.NewClient(svs.SessionManager)
		if err != nil {
			return nil, err
		}

		factory.sessionManager = sessionManager
	}

	return factory, nil
}

func (f *Factory) RegistryService() *registry.Service {
	return f.registryService
}

func (f *Factory) SessionManager() *sessionmanager.Client {
	return f.sessionManager
}

func (f *Factory) Close() error {
	var errs []error

	if f.registryService != nil {
		err := f.registryService.Close()
		errs = append(errs, err)
	}

	if f.sessionManager != nil {
		err := f.sessionManager.Close()
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}
