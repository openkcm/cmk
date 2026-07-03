package cmkpluginregistry

import "context"

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
