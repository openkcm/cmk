package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/openkcm/cmk/cmd/cmkd/commands"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/testutils"
	integrationutils "github.com/openkcm/cmk/test/integration/integration_utils"
)

func TestCommands(t *testing.T) {
	tests := []struct {
		name                    string
		commandFunc             func() *cobra.Command
		expectedUse             string
		doesNotHaveStatusServer bool
	}{
		{
			name:        "api-server",
			commandFunc: commands.NewAPIServer,
			expectedUse: "api-server",
		},
		{
			name:        "task-scheduler",
			commandFunc: commands.NewTaskScheduler,
			expectedUse: "task-scheduler",
		},
		{
			name:        "task-worker",
			commandFunc: commands.NewTaskWorker,
			expectedUse: "task-worker",
		},
		{
			name:        "tenant-manager",
			commandFunc: commands.NewTenantManager,
			expectedUse: "tenant-manager",
		},
		{
			name:        "event-reconciler",
			commandFunc: commands.NewEventReconciler,
			expectedUse: "event-reconciler",
		},
		{
			name:                    "db-migrator",
			commandFunc:             commands.NewDBMigrator,
			expectedUse:             "db-migrator",
			doesNotHaveStatusServer: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.commandFunc()

			assert.Equal(t, tt.expectedUse, cmd.Use)
			assert.NotEmpty(t, cmd.Short)
			assert.NotEmpty(t, cmd.Long)
			require.NotNil(t, cmd.RunE, "Command should have RunE function")

			port, err := testutils.GetFreePort()
			require.NoError(t, err, "failed to get free port")

			statusServer := createTestConfigFile(t, port)

			if tt.doesNotHaveStatusServer {
				err := cmd.RunE(cmd, []string{})
				require.NoError(t, err)
				return
			}

			errChan := make(chan error, 1)

			go func() {
				errChan <- cmd.RunE(cmd, []string{})
			}()

			// If status server gives back 200, service has started
			testutils.WaitForServer(t, statusServer)

			// Send interrupt signal to trigger graceful shutdown
			p, err := os.FindProcess(os.Getpid())
			require.NoError(t, err, "failed to get process")
			err = p.Signal(os.Interrupt)
			require.NoError(t, err, "failed to send interrupt signal")

			// Wait for the service to exit
			select {
			case err := <-errChan:
				assert.NoError(t, err, "Service should shut down gracefully without error")
			case <-time.After(5 * time.Second):
				assert.Fail(t, "Service did not shutdown within timeout")
			}
		})
	}
}

// createTestConfigFile creates a minimal test config file with the given status server port
func createTestConfigFile(t *testing.T, port int) string {
	t.Helper()

	_, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})
	_, amqpCfg := testutils.NewAMQPClient(t, testutils.AMQPCfg{})

	cfg := &config.Config{
		BaseConfig: commoncfg.BaseConfig{
			Application: commoncfg.Application{
				Name: "cmk-test",
			},
			Status: commoncfg.Status{
				Enabled: true,
				Address: fmt.Sprintf("localhost:%d", port),
			},
			Logger: commoncfg.Logger{
				Level: "error",
			},
		},
		Database: dbCfg,
		Certificates: config.Certificates{
			ValidityDays: config.MinCertificateValidityDays,
		},
		Services: config.Services{
			Registry:       testutils.TestRegistryConfig,
			SessionManager: testutils.TestSessionManagerConfig,
		},
		TenantManager: config.TenantManager{
			SecretRef: commoncfg.SecretRef{
				Type: commoncfg.InsecureSecretType,
			},
			AMQP: amqpCfg,
		},
		Plugins: integrationutils.NoopPluginConfigs(),
	}

	testutils.StartRedis(t, &cfg.Scheduler)

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	data, err := yaml.Marshal(cfg)
	require.NoError(t, err, "failed to marshal config")

	err = os.WriteFile("config.yaml", data, 0o600)
	require.NoError(t, err, "failed to write config file")

	return cfg.Status.Address
}
