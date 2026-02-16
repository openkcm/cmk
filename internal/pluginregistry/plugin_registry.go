package cmkpluginregistry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/plugin-sdk/pkg/catalog"

	servicewrapper "github.com/openkcm/plugin-sdk/service/wrapper"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk/internal/config"
)

var ErrNoPluginInCatalog = errors.New("no plugin in catalog")

// New creates a new instance of Catalog with the provided configuration.
func New(ctx context.Context, cfg *config.Config) (*Registry, error) {
	catalogLogger := slog.With("context", "plugin-catalog")

	svcRepo, err := servicewrapper.CreateServiceRepository(ctx, catalog.Config{
		Logger:        catalogLogger,
		PluginConfigs: cfg.Plugins,
	})
	if err != nil {
		catalogLogger.ErrorContext(ctx, "Error loading plugins", "error", err)
		return nil, fmt.Errorf("error loading plugins: %w", err)
	}

	pluginBuildInfos := make([]string, 0)
	for _, pluginInfo := range svcRepo.RawCatalog.ListPluginInfo() {
		pluginBuildInfos = append(pluginBuildInfos, pluginInfo.Build())
	}

	err = commoncfg.UpdateComponentsOfBuildInfo(&cfg.BaseConfig, pluginBuildInfos...)
	if err != nil {
		slogctx.Error(ctx, "Failed to update components of build info")
	}

	return &Registry{
		Registry: svcRepo,
		Catalog:  svcRepo.RawCatalog,
	}, nil
}
