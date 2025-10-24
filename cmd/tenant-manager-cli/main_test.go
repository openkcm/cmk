package main_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tmCLI "github.com/openkcm/cmk-core/cmd/tenant-manager-cli"
	"github.com/openkcm/cmk-core/internal/config"
)

var errTest = errors.New("test error")

const (
	validConfigContent = `application:
  name: test-app
logger:
  level: info`

	invalidConfigContent = `invalid: yaml: content: [
  malformed: yaml`
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

func TestRunFunctionWithSigHandling(t *testing.T) {
	t.Run("Should return exit code 1 when config loading fails", func(t *testing.T) {
		// Setup environment with invalid config to force loading failure
		setupTestEnvironment(t, invalidConfigContent)

		testFunc := func(_ context.Context, _ *config.Config) error {
			t.Error("Should not reach this point when config loading fails")
			return nil
		}

		exitCode := tmCLI.RunFunctionWithSigHandling(testFunc)
		assert.Equal(t, 1, exitCode)
	})

	t.Run("Should return exit code 1 when run function returns error", func(t *testing.T) {
		setupTestEnvironment(t, validConfigContent)

		testFunc := func(_ context.Context, _ *config.Config) error {
			return errTest
		}

		exitCode := tmCLI.RunFunctionWithSigHandling(testFunc)
		assert.Equal(t, 1, exitCode)
	})

	t.Run("Should return exit code 0 when run function succeeds", func(t *testing.T) {
		setupTestEnvironment(t, validConfigContent)

		executed := false
		testFunc := func(_ context.Context, cfg *config.Config) error {
			executed = true

			assert.Equal(t, "test-app", cfg.Application.Name)

			return nil
		}

		exitCode := tmCLI.RunFunctionWithSigHandling(testFunc)
		assert.Equal(t, 0, exitCode)
		assert.True(t, executed)
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
