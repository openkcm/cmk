package clients

import (
	"errors"
	"io"

	"github.com/openkcm/cmk/internal/clients/registry"
	sessionmanager "github.com/openkcm/cmk/internal/clients/session-manager"
	"github.com/openkcm/cmk/internal/config"
)

type Factory interface {
	io.Closer

	Registry() registry.Service
	SessionManager() sessionmanager.Service
}

type factory struct {
	registry       registry.Service
	sessionManager sessionmanager.Service

	cfg *config.Services
}

var _ Factory = (*factory)(nil)

func NewFactory(svs config.Services) (Factory, error) {
	factory := &factory{
		cfg: &svs,
	}

	if svs.Registry != nil && svs.Registry.Enabled {
		registryService, err := registry.NewService(svs.Registry)
		if err != nil {
			return nil, err
		}

		factory.registry = registryService
	}

	if svs.SessionManager != nil && svs.SessionManager.Enabled {
		sessionManager, err := sessionmanager.NewService(svs.SessionManager)
		if err != nil {
			return nil, err
		}

		factory.sessionManager = sessionManager
	}

	return factory, nil
}

func (f *factory) Registry() registry.Service {
	return f.registry
}

func (f *factory) SessionManager() sessionmanager.Service {
	return f.sessionManager
}

func (f *factory) Close() error {
	var errs []error

	if f.registry != nil {
		err := f.registry.Close()
		errs = append(errs, err)
	}

	if f.sessionManager != nil {
		err := f.sessionManager.Close()
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}
