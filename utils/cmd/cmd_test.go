package cmd_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/utils/cmd"
)

var ErrForced = errors.New("")

func buildCfg(t *testing.T) *config.Config {
	t.Helper()
	_, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})

	contextModels := &config.ContextModels{
		System: config.System{
			Identifier: config.SystemProperty{
				DisplayName: "GTID",
				Internal:    true,
			},
			Region: config.SystemProperty{
				DisplayName: "Region",
				Internal:    true,
			},
			Type: config.SystemProperty{
				DisplayName: "Type",
				Internal:    true,
			},
			OptionalProperties: map[string]config.SystemProperty{},
		},
	}

	bytes, err := yaml.Marshal(contextModels)
	require.NoError(t, err)

	return &config.Config{
		HTTP: config.HTTPServer{
			Address: "localhost:8082",
		},

		Database: dbCfg,
		BaseConfig: commoncfg.BaseConfig{
			Logger: commoncfg.Logger{
				Format: "json",
				Level:  "info",
			},
		},
		ConfigurableContext: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  string(bytes),
		},
		Certificates: config.Certificates{
			ValidityDays: config.MinCertificateValidityDays,
		},
	}
}

func TestRunFunctionWithSigHandling(t *testing.T) {
	t.Run("Should exitCode 1 when run function returns error", func(t *testing.T) {
		// Run in a temp directory for isolation
		tempDir := t.TempDir()
		t.Chdir(tempDir)

		// Create a minimal valid config so LoadConfig succeeds
		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				Logger: commoncfg.Logger{
					Format: "json",
					Level:  "info",
				},
			},
		}
		bytes, err := yaml.Marshal(cfg)
		require.NoError(t, err)
		err = os.WriteFile("config.yaml", bytes, 0o600)
		require.NoError(t, err)

		// The function returns an error
		exitCode := cmd.RunFuncWithSignalHandling(func(ctx context.Context, c *config.Config) error {
			return ErrForced
		}, cmd.RunFlags{})

		require.Equal(t, 1, exitCode)
	})

	tests := []struct {
		name     string
		cfg      func() *config.Config
		exitCode int
	}{
		{
			name: "should exitCode 0 on successful run",
			cfg: func() *config.Config {
				return buildCfg(t)
			},
			exitCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run in a temp directory for isolation
			tempDir := t.TempDir()
			t.Chdir(tempDir)

			filename := "config.yaml"
			bytes, err := yaml.Marshal(tt.cfg())
			require.NoError(t, err)

			err = os.WriteFile(filename, bytes, 0o600)
			require.NoError(t, err)

			exitCode := cmd.RunFuncWithSignalHandling(func(_ context.Context, _ *config.Config) error {
				return nil
			}, cmd.RunFlags{})
			require.Equal(t, tt.exitCode, exitCode)
		})
	}
}
