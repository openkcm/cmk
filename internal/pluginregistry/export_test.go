package cmkpluginregistry

import (
	"context"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk/internal/config"
)

// Exported for testing only.

const (
	PluginFailureThreshold     = pluginFailureThreshold
	DefaultPluginWatchInterval = defaultPluginWatchInterval
)

func (w *PluginWatcher) SetShutdown(fn func(err error)) {
	w.shutdown = fn
}

func (w *PluginWatcher) FailureCounts() map[string]int {
	return w.failureCounts
}

func (w *PluginWatcher) Check(ctx context.Context) {
	w.check(ctx)
}

func BuildCMKConfigOverrides(cfg *config.Config) (map[string]map[string]any, error) {
	return buildCMKConfigOverrides(cfg)
}

func MergePluginConfigsWithCMKConfigs(
	plugins []plugincatalog.PluginConfig,
	overrides map[string]map[string]any,
) ([]plugincatalog.PluginConfig, error) {
	return mergePluginConfigsWithCMKConfigs(plugins, overrides)
}
