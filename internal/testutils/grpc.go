package testutils

import (
	"net"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commongrpc"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"github.com/openkcm/cmk/internal/clients"
)

const (
	DefaultThrottleInterval = 5 * time.Second
)

// ServiceRegistrar registers a service with a gRPC server
type ServiceRegistrar func(*grpc.Server)

// TestRegistryConfig is a default registry config for testing purposes
var TestRegistryConfig = &commoncfg.GRPCClient{
	Enabled: true,
	Address: "localhost:9092",
	SecretRef: &commoncfg.SecretRef{
		Type: commoncfg.InsecureSecretType,
	},
}

// NewGRPCSuite creates a new gRPC server and client connection for testing purposes.
// Returns the server and client connection for use in tests.
func NewGRPCSuite(
	tb testing.TB,
	registrars ...ServiceRegistrar,
) (
	*grpc.Server,
	*commongrpc.DynamicClientConn,
) {
	tb.Helper()

	port, err := GetFreePortString()
	assert.NoError(tb, err)

	cfg := commoncfg.GRPCClient{
		Enabled: true,
		Address: "localhost:" + port,
	}

	grpcServer := NewGRPCServer(tb, cfg.Address, registrars...)

	grpcClient, err := commongrpc.NewDynamicClientConn(&cfg, DefaultThrottleInterval)
	assert.NoError(tb, err)

	return grpcServer, grpcClient
}

// NewGRPCServer is mostly used for internal reasons.
// In most cases please use NewGRPCSuite to set up a DB for unit tests
func NewGRPCServer(
	tb testing.TB,
	address string,
	registrars ...ServiceRegistrar,
) *grpc.Server {
	tb.Helper()

	grpcServer := grpc.NewServer()
	for _, register := range registrars {
		register(grpcServer)
	}

	lc := net.ListenConfig{}
	lis, err := lc.Listen(tb.Context(), "tcp", address)

	// Handle server shutdown gracefully when the process is terminated.
	// This also guarantees coverage reporting during integration tests.
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		grpcServer.GracefulStop()
	}()

	go func() {
		err = grpcServer.Serve(lis)
		if err != nil {
			tb.Fail()
		}
	}()

	return grpcServer
}

func CloseClientsFactory(t *testing.T, clientsFactory *clients.Factory) {
	t.Helper()

	err := clientsFactory.Close()
	assert.NoError(t, err)
}
