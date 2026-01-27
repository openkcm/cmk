package cmd_test

import (
	"context"
	"os"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/utils/cmd"
)

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
	t.Run("Should exitCode 1 on config not found", func(t *testing.T) {
		exitCode := cmd.RunFuncWithSignalHandling(func(ctx context.Context, c *config.Config) error {
			return nil
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
			filename := "config.yaml"
			f, err := os.Create(filename)
			require.NoError(t, err)

			bytes, err := yaml.Marshal(tt.cfg())
			require.NoError(t, err)

			_, err = f.Write(bytes)
			require.NoError(t, err)

			defer f.Close()
			defer os.Remove(filename)

			exitCode := cmd.RunFuncWithSignalHandling(func(_ context.Context, _ *config.Config) error {
				return nil
			}, cmd.RunFlags{})
			require.Equal(t, tt.exitCode, exitCode)
		})
	}
}
