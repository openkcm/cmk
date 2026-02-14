package cmkplugincatalog

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/plugin-sdk/pkg/catalog"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk/internal/config"
)

// New creates a new instance of Catalog with the provided configuration.
func New(ctx context.Context, cfg *config.Config) (*Registry, error) {
	catalogLogger := slog.With("context", "plugin-catalog")
	catalogConfig := catalog.Config{
		Logger:        catalogLogger,
		PluginConfigs: cfg.Plugins,
	}

	catalog, err := catalog.Load(ctx, catalogConfig)
	if err != nil {
		catalogLogger.ErrorContext(ctx, "Error loading plugins", "error", err)
		return nil, fmt.Errorf("error loading plugins: %w", err)
	}

	pluginBuildInfos := make([]string, 0)
	for _, pluginInfo := range catalog.ListPluginInfo() {
		pluginBuildInfos = append(pluginBuildInfos, pluginInfo.Build())
	}

	err = commoncfg.UpdateComponentsOfBuildInfo(&cfg.BaseConfig, pluginBuildInfos...)
	if err != nil {
		slogctx.Error(ctx, "Failed to update components of build info")
	}

	c := NewPluginCatalog(catalog)
	//err = c.Validate()
	//if err != nil {
	//	return nil, err
	//}

	return c, nil
}
