package main_test

import (
	"errors"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
