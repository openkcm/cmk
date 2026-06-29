package main

import (
	"os"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/openkcm/cmk/cmd/cmkctl/commands"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/testutils"
	integrationutils "github.com/openkcm/cmk/test/integration/integration_utils"
)

func TestCommandsWithConfig(t *testing.T) {
	tests := []struct {
		name        string
		commandFunc func() *cobra.Command
		expectedUse string
	}{
		{
			name:        "tenant-manager-cli",
			commandFunc: commands.NewTenantManagerCLI,
			expectedUse: "tenant",
		},
		{
			name:        "task-cli",
			commandFunc: commands.NewTaskCLI,
			expectedUse: "task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.commandFunc()

			assert.Equal(t, tt.expectedUse, cmd.Use)
			assert.NotEmpty(t, cmd.Short)

			createTestConfigFileForCLI(t)

			_ = cmd.PersistentPreRunE(cmd, []string{})
		})
	}
}

// createTestConfigFileForCLI creates a minimal test config file for CLI commands
func createTestConfigFileForCLI(t *testing.T) {
	t.Helper()

	_, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})
	_, amqpCfg := testutils.NewAMQPClient(t, testutils.AMQPCfg{})

	cfg := &config.Config{
		BaseConfig: commoncfg.BaseConfig{
			Application: commoncfg.Application{
				Name: "cmkctl-test",
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
}
