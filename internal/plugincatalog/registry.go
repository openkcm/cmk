package cmkplugincatalog

import (
	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
	serviceapi "github.com/openkcm/plugin-sdk/service/api"
)

type Registry struct {
	serviceapi.Registry
	*plugincatalog.Catalog
}

func (p *Registry) Close() error {
	return p.Registry.Close()
}
