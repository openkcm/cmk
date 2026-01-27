package main_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tenantmanager "github.com/openkcm/cmk/cmd/tenant-manager"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/testutils"
)

var ErrTest = errors.New("test error")

func TestLoadConfig(t *testing.T) {
	t.Run("Should return error if config file not found", func(t *testing.T) {
		_, err := tenantmanager.LoadConfig()
		require.Error(t, err)
	})

	t.Run("Should return error if config file has wrong struct", func(t *testing.T) {
		content := []byte("application:\n  nameE: test-app\n")
		err := os.WriteFile("config.yaml", content, 0o600)
		require.NoError(t, err)

		defer os.Remove("config.yaml")

		_, err = tenantmanager.LoadConfig()
		require.Error(t, err)
	})

	t.Run("Should load config if config file exists", func(t *testing.T) {
		content := []byte(`
application:
  name: test-app
logger:
  level: info
tenantManager:
  secretRef:
    type: insecure
  amqp:
    url: amqp://guest:guest@localhost:5672/
    target: cmk.global.tenants
    source: cmk.emea.tenants
services:
  registry:
    enabled: true
  sessionManager:
    enabled: true
`)

		err := os.WriteFile("config.yaml", content, 0o600)
		require.NoError(t, err)

		defer os.Remove("config.yaml")

		cfg, err := tenantmanager.LoadConfig()
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.Equal(t, "test-app", cfg.Application.Name)
	})
}

func TestRunFuncWithSignalHandling(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		exitCode := tenantmanager.RunFunctionWithSigHandling(func(_ context.Context, _ *config.Config) error {
			filename := "config.yaml"
			f, err := os.Create(filename)
			require.NoError(t, err)

			defer f.Close()
			defer os.Remove(filename)

			return nil
		})

		assert.Equal(t, 1, exitCode)
	})

	t.Run("error", func(t *testing.T) {
		exitCode := tenantmanager.RunFunctionWithSigHandling(func(_ context.Context, _ *config.Config) error {
			return ErrTest
		})

		assert.Equal(t, 1, exitCode)
	})
}

func TestStartStatusServer(t *testing.T) {
	cfg := &config.Config{}

	// Call and check for no panic.
	tenantmanager.StartStatusServer(t.Context(), cfg)

	// Optionally, wait a short time to let the goroutine start
	time.Sleep(100 * time.Millisecond)
}

func TestBusinessMain(t *testing.T) {
	_, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})
	_, amqpCfg := testutils.NewAMQPClient(t, testutils.AMQPCfg{})
	tests := []struct {
		name        string
		config      func() *config.Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "no amqp connection",
			config: func() *config.Config {
				return &config.Config{
					TenantManager: config.TenantManager{
						SecretRef: commoncfg.SecretRef{
							Type: commoncfg.InsecureSecretType,
						},
					}, // No AMQP connection info provided
					Database: dbCfg,
					Services: config.Services{
						Registry:       testutils.TestRegistryConfig,
						SessionManager: testutils.TestSessionManagerConfig,
					},
					BaseConfig: testutils.TestBaseConfig,
				}
			},
			expectError: true,
			errorMsg:    "Expected error due to missing AMQP connection info",
		},
		{
			name: "no db connection",
			config: func() *config.Config {
				return &config.Config{
					TenantManager: config.TenantManager{
						SecretRef: commoncfg.SecretRef{
							Type: commoncfg.InsecureSecretType,
						},
						AMQP: amqpCfg,
					},
					Database: config.Database{}, // No database connection info provided
					Services: config.Services{
						Registry:       testutils.TestRegistryConfig,
						SessionManager: testutils.TestSessionManagerConfig,
					},
					BaseConfig: testutils.TestBaseConfig,
				}
			},
			expectError: true,
			errorMsg:    "Expected error due to missing database configuration",
		},
		{
			name: "no grpc configuration",
			config: func() *config.Config {
				return &config.Config{
					TenantManager: config.TenantManager{
						SecretRef: commoncfg.SecretRef{
							Type: commoncfg.InsecureSecretType,
						},
						AMQP: amqpCfg,
					},
					Database:   dbCfg,
					Services:   config.Services{}, // No gRPC configuration provided
					BaseConfig: testutils.TestBaseConfig,
				}
			},
			expectError: true,
			errorMsg:    "Expected error due to missing gRPC configuration",
		},
		{
			name: "missing registry service configuration",
			config: func() *config.Config {
				return &config.Config{
					TenantManager: config.TenantManager{
						SecretRef: commoncfg.SecretRef{
							Type: commoncfg.InsecureSecretType,
						},
						AMQP: amqpCfg,
					},
					Database: dbCfg,
					Services: config.Services{
						// Missing Registry configuration
						SessionManager: testutils.TestSessionManagerConfig,
					},
					BaseConfig: testutils.TestBaseConfig,
				}
			},
			expectError: true,
			errorMsg:    "registry service configuration is required",
		},
		{
			name: "missing session-manager service configuration",
			config: func() *config.Config {
				return &config.Config{
					TenantManager: config.TenantManager{
						SecretRef: commoncfg.SecretRef{
							Type: commoncfg.InsecureSecretType,
						},
						AMQP: amqpCfg,
					},
					Database: dbCfg,
					Services: config.Services{
						Registry: testutils.TestRegistryConfig,
						// Missing SessionManager configuration
					},
					BaseConfig: testutils.TestBaseConfig,
				}
			},
			expectError: true,
			errorMsg:    "session-manager service configuration is required",
		},
		{
			name: "valid configuration",
			config: func() *config.Config {
				return &config.Config{
					TenantManager: config.TenantManager{
						SecretRef: commoncfg.SecretRef{
							Type: commoncfg.InsecureSecretType,
						},
						AMQP: amqpCfg,
					},
					Database: dbCfg,
					Services: config.Services{
						Registry:       testutils.TestRegistryConfig,
						SessionManager: testutils.TestSessionManagerConfig,
					},
					BaseConfig: commoncfg.BaseConfig{
						Logger: commoncfg.Logger{
							Format: "json",
							Level:  "info",
						},
					},
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			err := tenantmanager.Run(ctx, tt.config())

			if tt.expectError {
				assert.Error(t, err, tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
