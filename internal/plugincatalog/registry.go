package cmkplugincatalog

import (
	"github.com/openkcm/plugin-sdk/pkg/catalog"

	serviceapi "github.com/openkcm/plugin-sdk/api/service"
)

type Registry struct {
	serviceapi.Registry
	catalog.Catalog
}

func (p *Registry) Close() error {
	return p.Registry.Close()
}
