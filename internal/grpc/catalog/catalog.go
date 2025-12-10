package catalog

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/plugin-sdk/api"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
	slogctx "github.com/veqryn/slog-context"

	"github.tools.sap/kms/cmk/internal/config"
)

// New creates a new instance of Catalog with the provided configuration.
func New(ctx context.Context, cfg *config.Config) (*plugincatalog.Catalog, error) {
	catalogLogger := slog.With("context", "plugin-catalog")
	catalogConfig := plugincatalog.Config{
		Logger:        catalogLogger,
		PluginConfigs: cfg.Plugins,
		HostServices:  []api.ServiceServer{},
	}

	catalog, err := plugincatalog.Load(ctx, catalogConfig)
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

	return catalog, nil
}
