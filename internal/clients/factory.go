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
}

var _ Factory = (*factory)(nil)

func NewFactory(cfg *config.Config) (Factory, error) {
	factory := &factory{}

	if cfg.Services.Registry != nil && cfg.Services.Registry.Enabled {
		registryService, err := registry.NewService(cfg.Services.Registry, cfg.ContextModels.System)
		if err != nil {
			return nil, err
		}

		factory.registry = registryService
	}

	if cfg.Services.SessionManager != nil && cfg.Services.SessionManager.Enabled {
		sessionManager, err := sessionmanager.NewService(cfg.Services.SessionManager)
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
