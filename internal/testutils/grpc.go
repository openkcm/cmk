package testutils

import (
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commongrpc"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
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

// TestSessionManagerConfig is a default session manager config for testing purposes
var TestSessionManagerConfig = &commoncfg.GRPCClient{
	Enabled: true,
	Address: "localhost:9091",
	SecretRef: &commoncfg.SecretRef{
		Type: commoncfg.InsecureSecretType,
	},
}

var TestBaseConfig = commoncfg.BaseConfig{
	Logger: commoncfg.Logger{
		Format: "json",
		Level:  "info",
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

	grpcServer := grpc.NewServer()
	for _, register := range registrars {
		register(grpcServer)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	lc := net.ListenConfig{}
	lis, err := lc.Listen(tb.Context(), "tcp", cfg.Address)
	assert.NoError(tb, err)

	go func(lis net.Listener) {
		defer wg.Done()

		// GracefulStop in tb.Cleanup makes Serve return ErrServerStopped on
		// some grpc-go versions; that is the expected shutdown path, not a
		// test failure.
		if serveErr := grpcServer.Serve(lis); serveErr != nil && !errors.Is(serveErr, grpc.ErrServerStopped) {
			assert.NoError(tb, serveErr)
		}
	}(lis)

	grpcClient, err := commongrpc.NewDynamicClientConn(&cfg, DefaultThrottleInterval)
	assert.NoError(tb, err)

	tb.Cleanup(func() {
		grpcServer.GracefulStop()
		wg.Wait()

		err := grpcClient.Close()
		assert.NoError(tb, err)
	})

	return grpcServer, grpcClient
}
