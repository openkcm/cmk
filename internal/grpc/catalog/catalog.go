package catalog

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/openkcm/plugin-sdk/api"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/plugins/builtin"
)

// New creates a new instance of Catalog with the provided configuration.
func New(ctx context.Context, cfg config.Config) (*plugincatalog.Catalog, error) {
	catalogLogger := slog.With("context", "plugin-catalog")
	catalogConfig := plugincatalog.Config{
		Logger:        catalogLogger,
		PluginConfigs: cfg.Plugins,
		HostServices:  []api.ServiceServer{},
	}

	catalog, err := plugincatalog.Load(ctx, catalogConfig, builtin.BuiltIns()...)
	if err != nil {
		catalogLogger.ErrorContext(ctx, "Error loading plugins", "error", err)
		return nil, fmt.Errorf("error loading plugins: %w", err)
	}

	return catalog, nil
}
