package cmkpluginregistry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"syscall"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	pluginapi "github.com/openkcm/plugin-sdk/api"
	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
)

const (
	defaultPluginWatchInterval = 30 * time.Second
	pluginPingTimeout          = 5 * time.Second
	// pluginFailureThreshold is the number of consecutive failures before shutdown is triggered.
	pluginFailureThreshold = 3

	// goPluginHealthService is the service name go-plugin registers with the gRPC health server.
	goPluginHealthService = "plugin"
)

var (
	ErrPluginCheckFailed = errors.New("plugin health check failed")
)

// pluginLister is the subset of *catalog.Catalog used by PluginWatcher.
type pluginLister interface {
	ListPluginInfo() []pluginapi.Info
	LookupByTypeAndName(pluginType, pluginName string) plugincatalog.Plugin
}

type PluginWatcher struct {
	catalog       pluginLister
	interval      time.Duration
	shutdown      func(err error)
	failureCounts map[string]int
}

func NewPluginWatcher(catalog pluginLister, interval time.Duration) *PluginWatcher {
	if interval <= 0 {
		interval = defaultPluginWatchInterval
	}
	return &PluginWatcher{
		catalog:       catalog,
		interval:      interval,
		failureCounts: make(map[string]int),
		shutdown: func(err error) {
			_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		},
	}
}

func (w *PluginWatcher) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.check(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.check(ctx)
		}
	}
}

func (w *PluginWatcher) check(ctx context.Context) {
	for _, info := range w.catalog.ListPluginInfo() {
		plugin := w.catalog.LookupByTypeAndName(info.Type(), info.Name())
		if plugin == nil {
			continue
		}

		key := info.Type() + "/" + info.Name()

		if err := w.ping(ctx, plugin); err != nil {
			w.failureCounts[key]++
			log.Warn(ctx, "Plugin health check failed",
				slog.String("plugin", info.Name()),
				slog.String("type", info.Type()),
				slog.Int("consecutiveFailures", w.failureCounts[key]),
				slog.Int("threshold", pluginFailureThreshold),
				log.ErrorAttr(err),
			)
			if w.failureCounts[key] >= pluginFailureThreshold {
				log.Error(ctx, "Plugin health check threshold reached, initiating shutdown",
					err,
					slog.String("plugin", info.Name()),
					slog.String("type", info.Type()),
				)
				w.shutdown(err)
				return
			}
		} else {
			w.failureCounts[key] = 0
		}
	}
}

func (w *PluginWatcher) ping(ctx context.Context, plugin plugincatalog.Plugin) error {
	pingCtx, cancel := context.WithTimeout(ctx, pluginPingTimeout)
	defer cancel()

	client := grpc_health_v1.NewHealthClient(plugin.ClientConnection())
	resp, err := client.Check(pingCtx, &grpc_health_v1.HealthCheckRequest{Service: goPluginHealthService})
	if err != nil {
		// Built-in plugins use an in-process server that doesn't register the health service.
		if status.Code(err) == codes.Unimplemented {
			return nil
		}
		return errs.Wrap(ErrPluginCheckFailed, err)
	}
	if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		return errs.Wrapf(ErrPluginCheckFailed, fmt.Sprintf("plugin %s is not serving", plugin.Info().Name()))
	}
	return nil
}
