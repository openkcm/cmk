package main_test

import (
	"context"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

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

func TestRunExecutesCommands(t *testing.T) {
	ctx := context.Background()
	err := taskCLI.Run(ctx, buildCfg(t))
	assert.NoError(t, err)
}
