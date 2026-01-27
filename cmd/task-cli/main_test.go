package main_test

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	taskCLI "github.com/openkcm/cmk/cmd/task-cli"
	"github.com/openkcm/cmk/internal/config"
)

func buildCfg(t *testing.T) *config.Config {
	t.Helper()

	return &config.Config{
		BaseConfig: commoncfg.BaseConfig{
			Logger: commoncfg.Logger{
				Format: "json",
				Level:  "info",
			},
			Application: commoncfg.Application{
				Name: "test-async",
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
	}
}

func writeCfg(t *testing.T, cfg *config.Config, filename string) *os.File {
	t.Helper()

	file, err := os.Create(filename)
	require.NoError(t, err)

	bytes, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	_, err = file.Write(bytes)
	require.NoError(t, err)

	return file
}

func TestRunFuncHandlesSignal(t *testing.T) {
	cfg := buildCfg(t)
	filename := "config.yaml"
	file := writeCfg(t, cfg, filename)

	defer file.Close()
	defer os.Remove(filename)

	signal.Ignore(syscall.SIGTERM)
	defer signal.Reset(syscall.SIGTERM)

	called := false
	f := func(ctx context.Context, cfg *config.Config) error {
		called = true
		return nil
	}

	exitCode := taskCLI.RunFunctionWithSigHandling(f)
	assert.Equal(t, 0, exitCode)
	assert.True(t, called)
}

func TestRunFuncHandlesSignalFailure(t *testing.T) {
	signal.Ignore(syscall.SIGTERM)
	defer signal.Reset(syscall.SIGTERM)

	f := func(ctx context.Context, cfg *config.Config) error {
		return os.ErrClosed
	}

	exitCode := taskCLI.RunFunctionWithSigHandling(f)
	assert.NotEqual(t, 0, exitCode)
}

func TestRunExecutesCommands(t *testing.T) {
	ctx := context.Background()
	err := taskCLI.Run(ctx, buildCfg(t))
	assert.NoError(t, err)
}
