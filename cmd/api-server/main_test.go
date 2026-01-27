package main_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	apiServer "github.com/openkcm/cmk/cmd/api-server"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/testutils"
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

func TestServerRunningAndShutdown(t *testing.T) {
	ctx := t.Context()
	cfg := buildCfg(t)

	go func(ctx context.Context) {
		err := apiServer.Run(ctx, cfg)
		assert.NoError(t, err)
	}(ctx)

	url := "http://" + cfg.HTTP.Address + "/keys"

	// Wait until server has started
	for {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)

		r, err := http.DefaultClient.Do(req)
		if err == nil {
			defer r.Body.Close()
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	// Send shutdown to the server
	ctx.Done()

	// Wait until cant connect to the server
	for {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)

		r, err := http.DefaultClient.Do(req)
		if err == nil {
			defer r.Body.Close()
			break
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func TestRun(t *testing.T) {
	t.Run("Should error on not possible database connection", func(t *testing.T) {
		err := apiServer.Run(t.Context(), &config.Config{
			HTTP: config.HTTPServer{
				Address: "localhost:8082",
			},
			Database: config.Database{
				Host: commoncfg.SourceRef{
					Value: "error",
				},
				User: commoncfg.SourceRef{
					Value: "error",
				},
				Secret: commoncfg.SourceRef{
					Value: "error",
				},
				Name: "error",
				Port: "5433",
			},
			BaseConfig: commoncfg.BaseConfig{
				Logger: commoncfg.Logger{
					Format: "json",
					Level:  "info",
				},
			},
		})
		require.Error(t, err)
	})
}

func TestMonitorKeystorePoolSize(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cfg := buildCfg(t)
	cfg.KeystorePool = config.KeystorePool{
		Interval: 100 * time.Millisecond,
	}

	// Run in goroutine, should exit after context timeout
	go func() {
		apiServer.MonitorKeystorePoolSize(ctx, cfg)
	}()

	<-ctx.Done()
	// Check if the error is due to context deadline exceeded, not due to other reasons
	require.Error(t, ctx.Err(), &context.DeadlineExceeded)
}
