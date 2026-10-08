package cmkpluginregistry_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	pluginapi "github.com/openkcm/plugin-sdk/api"
	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
)

var errConnectionRefused = errors.New("connection refused")

// fakePlugin implements plugincatalog.Plugin with a configurable ClientConnection.
type fakePlugin struct {
	info pluginapi.Info
	conn grpc.ClientConnInterface
}

func (f *fakePlugin) Close() error                               { return nil }
func (f *fakePlugin) ClientConnection() grpc.ClientConnInterface { return f.conn }
func (f *fakePlugin) Info() pluginapi.Info                       { return f.info }
func (f *fakePlugin) Logger() *slog.Logger                       { return slog.Default() }
func (f *fakePlugin) GrpcServiceNames() []string                 { return nil }

var _ plugincatalog.Plugin = (*fakePlugin)(nil)

// fakeInfo implements pluginapi.Info.
type fakeInfo struct {
	name string
	typ  string
}

func (f fakeInfo) Name() string   { return f.name }
func (f fakeInfo) Type() string   { return f.typ }
func (f fakeInfo) Tags() []string { return nil }
func (f fakeInfo) Build() string  { return "" }
func (f fakeInfo) Version() uint  { return 1 }

var _ pluginapi.Info = fakeInfo{}

// fakeCatalog implements the pluginLister interface via ListPluginInfo/LookupByTypeAndName.
type fakeCatalog struct {
	plugins map[string]plugincatalog.Plugin
	infos   []pluginapi.Info
}

func (f *fakeCatalog) ListPluginInfo() []pluginapi.Info { return f.infos }
func (f *fakeCatalog) LookupByTypeAndName(typ, name string) plugincatalog.Plugin {
	return f.plugins[typ+"/"+name]
}

func newFakeCatalog(plugins ...*fakePlugin) *fakeCatalog {
	c := &fakeCatalog{plugins: make(map[string]plugincatalog.Plugin)}
	for _, p := range plugins {
		c.infos = append(c.infos, p.info)
		c.plugins[p.info.Type()+"/"+p.info.Name()] = p
	}
	return c
}

// fakeConn is a grpc.ClientConnInterface that delegates health checks to a handler func.
type fakeConn struct {
	grpc.ClientConnInterface

	handler func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error
}

func (f *fakeConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	return f.handler(ctx, method, args, reply, nil, opts...)
}

func (f *fakeConn) NewStream(
	_ context.Context,
	_ *grpc.StreamDesc,
	_ string,
	_ ...grpc.CallOption,
) (grpc.ClientStream, error) {
	return nil, io.EOF
}

// healthyConn returns a conn whose health check always succeeds.
func healthyConn() *fakeConn {
	return &fakeConn{
		handler: func(_ context.Context, _ string, _ any, reply any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			if r, ok := reply.(*grpc_health_v1.HealthCheckResponse); ok {
				r.Status = grpc_health_v1.HealthCheckResponse_SERVING
			}
			return nil
		},
	}
}

// failingConn returns a conn whose health check always returns the given error.
func failingConn(err error) *fakeConn {
	return &fakeConn{
		handler: func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			return err
		},
	}
}

// notServingConn returns a conn whose health check responds NOT_SERVING.
func notServingConn() *fakeConn {
	return &fakeConn{
		handler: func(_ context.Context, _ string, _ any, reply any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			if r, ok := reply.(*grpc_health_v1.HealthCheckResponse); ok {
				r.Status = grpc_health_v1.HealthCheckResponse_NOT_SERVING
			}
			return nil
		},
	}
}

func newWatcher(catalog *fakeCatalog) (*cmkpluginregistry.PluginWatcher, *int) {
	w := cmkpluginregistry.NewPluginWatcher(catalog, cmkpluginregistry.DefaultPluginWatchInterval)
	shutdownCount := 0
	w.SetShutdown(func(err error) { shutdownCount++ })
	return w, &shutdownCount
}

func TestPluginWatcher_HealthyPlugin(t *testing.T) {
	plugin := &fakePlugin{info: fakeInfo{name: "kms", typ: "KeyManagement"}, conn: healthyConn()}
	w, shutdownCount := newWatcher(newFakeCatalog(plugin))

	w.Check(t.Context())

	assert.Equal(t, 0, *shutdownCount)
	assert.Equal(t, 0, w.FailureCounts()["KeyManagement/kms"])
}

func TestPluginWatcher_SingleFailureNoShutdown(t *testing.T) {
	plugin := &fakePlugin{info: fakeInfo{name: "kms", typ: "KeyManagement"}, conn: failingConn(errConnectionRefused)}
	w, shutdownCount := newWatcher(newFakeCatalog(plugin))

	w.Check(t.Context())

	assert.Equal(t, 0, *shutdownCount, "should not shutdown on first failure")
	assert.Equal(t, 1, w.FailureCounts()["KeyManagement/kms"])
}

func TestPluginWatcher_ThresholdTriggersShutdown(t *testing.T) {
	plugin := &fakePlugin{info: fakeInfo{name: "kms", typ: "KeyManagement"}, conn: failingConn(errConnectionRefused)}
	w, shutdownCount := newWatcher(newFakeCatalog(plugin))

	for range cmkpluginregistry.PluginFailureThreshold {
		w.Check(t.Context())
	}

	assert.Equal(t, 1, *shutdownCount)
	assert.Equal(t, cmkpluginregistry.PluginFailureThreshold, w.FailureCounts()["KeyManagement/kms"])
}

func TestPluginWatcher_FailureCounterResetsAfterRecovery(t *testing.T) {
	plugin := &fakePlugin{info: fakeInfo{name: "kms", typ: "KeyManagement"}, conn: failingConn(errConnectionRefused)}
	w, shutdownCount := newWatcher(newFakeCatalog(plugin))

	// Two failures — below threshold
	w.Check(t.Context())
	w.Check(t.Context())
	require.Equal(t, 2, w.FailureCounts()["KeyManagement/kms"])

	// Recover
	plugin.conn = healthyConn()
	w.Check(t.Context())

	assert.Equal(t, 0, *shutdownCount, "should not shutdown after recovery")
	assert.Equal(t, 0, w.FailureCounts()["KeyManagement/kms"], "counter should reset")
}

func TestPluginWatcher_UnimplementedIsSkipped(t *testing.T) {
	unimplErr := status.Error(codes.Unimplemented, "unknown service grpc.health.v1.Health")
	plugin := &fakePlugin{info: fakeInfo{name: "noop", typ: "IdentityManagement"}, conn: failingConn(unimplErr)}
	w, shutdownCount := newWatcher(newFakeCatalog(plugin))

	for range cmkpluginregistry.PluginFailureThreshold {
		w.Check(t.Context())
	}

	assert.Equal(t, 0, *shutdownCount, "Unimplemented should never trigger shutdown")
	assert.Equal(t, 0, w.FailureCounts()["IdentityManagement/noop"], "Unimplemented should not increment counter")
}

func TestPluginWatcher_NotServingTriggersShutdown(t *testing.T) {
	plugin := &fakePlugin{info: fakeInfo{name: "kms", typ: "KeyManagement"}, conn: notServingConn()}
	w, shutdownCount := newWatcher(newFakeCatalog(plugin))

	for range cmkpluginregistry.PluginFailureThreshold {
		w.Check(t.Context())
	}

	assert.Equal(t, 1, *shutdownCount)
}
