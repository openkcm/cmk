package main_test

import (
	"context"
	"os"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/assert"
	"gopkg.in/yaml.v3"

	taskWorker "github.com/openkcm/cmk/cmd/task-worker"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/testutils"
)

func buildCfg(t *testing.T) *config.Config {
	t.Helper()

	_, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})

	return &config.Config{
		BaseConfig: commoncfg.BaseConfig{
			Logger: commoncfg.Logger{
				Format: "json",
				Level:  "info",
			},
			Status: commoncfg.Status{
				Enabled:   true,
				Address:   ":8888",
				Profiling: false,
			},
		},
		Scheduler: config.Scheduler{
			TaskQueue: config.Redis{
				Host: commoncfg.SourceRef{Value: "test", Source: commoncfg.EmbeddedSourceValue},
				Port: "1234",

				ACL: config.RedisACL{
					Username: commoncfg.SourceRef{Value: "test", Source: commoncfg.EmbeddedSourceValue},
					Password: commoncfg.SourceRef{Value: "test", Source: commoncfg.EmbeddedSourceValue},
				},
				SecretRef: commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		},
		Certificates: config.Certificates{
			ValidityDays: config.MinCertificateValidityDays,
		},
		Database: dbCfg,
		Services: config.Services{
			Registry: testutils.TestRegistryConfig,
		},
	}
}

func TestExitSignal(t *testing.T) {
	t.Run("Should exitCode 1 on config not found", func(t *testing.T) {
		exitCode := taskWorker.RunFunctionWithSigHandling(func(_ context.Context, _ *config.Config) error {
			return nil
		})
		assert.Equal(t, 1, exitCode)
	})

	tests := []struct {
		name     string
		cfg      func(t *testing.T) *config.Config
		exitCode int
	}{
		{
			name:     "Should exitCode 0 on valid config without app startup",
			cfg:      buildCfg,
			exitCode: 0,
		},
		{
			name: "Should exitCode 1 on app startup fail",
			cfg: func(t *testing.T) *config.Config {
				t.Helper()

				cfg := buildCfg(t)
				cfg.Scheduler.TaskQueue = config.Redis{}

				return cfg
			},
			exitCode: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := "config.yaml"
			f, err := os.Create(filename)
			require.NoError(t, err)
			defer f.Close()
			defer os.Remove(filename)

			bytes, err := yaml.Marshal(tt.cfg(t))
			require.NoError(t, err)

			_, err = f.Write(bytes)
			require.NoError(t, err)

			exitCode := taskWorker.RunFunctionWithSigHandling(func(ctx context.Context, cfg *config.Config) error {
				if tt.exitCode != 0 {
					return taskWorker.Run(ctx, cfg)
				}

				return nil
			})
			assert.Equal(t, tt.exitCode, exitCode)
		})
	}
}
