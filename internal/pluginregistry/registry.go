package cmkpluginregistry

import (
	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
)

type Registry struct {
	serviceapi.Registry
	*plugincatalog.Catalog
}

func (p *Registry) Close() error {
	return p.Registry.Close()
}
