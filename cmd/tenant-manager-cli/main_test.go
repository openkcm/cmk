package main_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	tmCLI "github.com/openkcm/cmk/cmd/tenant-manager-cli"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/testutils"
)

var errTest = errors.New("test error")

func TestSetupCommands(t *testing.T) {
	t.Run("Should create root command with all subcommands", func(t *testing.T) {
		ctx := t.Context()

		_, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		svcRegistry := testutils.NewTestPlugins()

		cfg := &config.Config{
			Database: dbCfg,
		}

		rootCmd, err := tmCLI.SetupCommands(ctx, cfg, nil, svcRegistry)
		assert.NoError(t, err)

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
	t.Run("Should return exit code 1 when run function returns error", func(t *testing.T) {
		testFunc := func(_ context.Context, _ *config.Config) error {
			return errTest
		}

		exitCode := tmCLI.RunFunctionWithSigHandling(testFunc)
		assert.Equal(t, 1, exitCode)
	})

	t.Run("Should return exit code 0 when run function succeeds", func(t *testing.T) {
		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				Application: commoncfg.Application{Name: "test-app"},
				Logger: commoncfg.Logger{
					Level:  "info",
					Format: "json",
				},
			},
			Certificates: config.Certificates{
				ValidityDays: config.MinCertificateValidityDays,
			},
		}

		configBytes, err := yaml.Marshal(cfg)
		require.NoError(t, err)
		err = os.WriteFile("config.yaml", configBytes, 0o600)
		require.NoError(t, err)
		defer os.Remove("config.yaml")

		testFunc := func(_ context.Context, cfg *config.Config) error {
			return nil
		}

		exitCode := tmCLI.RunFunctionWithSigHandling(testFunc)
		assert.Equal(t, 0, exitCode)
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
