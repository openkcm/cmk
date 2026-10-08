package cmkpluginregistry

import (
	"context"

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

// WatchPlugins starts the plugin health watcher as a background goroutine.
// On detecting a dead plugin it sends SIGTERM to trigger graceful shutdown.
func (p *Registry) WatchPlugins(ctx context.Context) {
	watcher := NewPluginWatcher(p.Catalog, defaultPluginWatchInterval)
	go watcher.Start(ctx)
}
