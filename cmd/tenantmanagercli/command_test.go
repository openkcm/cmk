package tenantmanagercli_test

import (
	"os"
	"testing"

	tmCLI "github.com/openkcm/cmk/cmd/tenantmanagercli"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/config"
)

func TestSetupCommands(t *testing.T) {
	t.Run("Should create root command with all subcommands", func(t *testing.T) {
		ctx := t.Context()

		rootCmd := tmCLI.SetupCommands(ctx, nil)

		assert.NotNil(t, rootCmd)
		assert.NotEmpty(t, rootCmd.Use)
		assert.NotNil(t, rootCmd.PersistentFlags().Lookup("sleep"))

		commands := rootCmd.Commands()
		t.Logf("Found %d commands", len(commands))

		for _, cmd := range commands {
			t.Logf("Command: %s", cmd.Name())
		}

		assert.GreaterOrEqual(t, len(commands), 1, "Root command should be created")
	})
}

func TestRun(t *testing.T) {
	t.Run("Should error on invalid logger config", func(t *testing.T) {
		ctx := t.Context()

		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				Logger: commoncfg.Logger{
					Level:  "invalid-level",
					Format: "invalid-format",
					Formatter: commoncfg.LoggerFormatter{
						Time: commoncfg.LoggerTime{
							Type:      "unix",
							Precision: "*#md1",
						},
					},
				},
			},
		}

		err := tmCLI.Run(ctx, cfg)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Failed to initialise the logger")
	})

	t.Run("Should error on invalid database config", func(t *testing.T) {
		ctx := t.Context()

		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				Logger: commoncfg.Logger{
					Level:  "info",
					Format: "json",
				},
			},
			Database: config.Database{},
		}

		err := tmCLI.Run(ctx, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Failed to initialise db connection")
	})
}

// Helper function to create a temporary test environment with config file
func setupTestEnvironment(t *testing.T, configContent string) {
	t.Helper()

	tempDir := t.TempDir()
	t.Chdir(tempDir)

	if configContent != "" {
		writeConfigFile(t, configContent)
	}
}

// Helper function to write config file
func writeConfigFile(t *testing.T, content string) {
	t.Helper()

	err := os.WriteFile("config.yaml", []byte(content), 0600)
	require.NoError(t, err)
}
