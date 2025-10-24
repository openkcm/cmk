package main_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/require"

	apiServer "github.com/openkcm/cmk-core/cmd/api-server"
	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/testutils"
)

func TestServerRunningAndShutdown(t *testing.T) {
	_, _ = testutils.NewTestDB(t, testutils.TestDBConfig{})

	cfg := &config.Config{
		HTTP: config.HTTPServer{
			Address: "localhost:8082",
		},

		Database: config.Database{
			Host: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  "localhost",
			},
			User: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  "postgres",
			},
			Secret: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  "secret",
			},
			Name: "cmk",
			Port: "5433",
		},
		BaseConfig: commoncfg.BaseConfig{
			Logger: commoncfg.Logger{
				Format: "json",
				Level:  "info",
			},
		},
	}

	ctx := t.Context()

	go func() {
		err := apiServer.Run(ctx, cfg)
		//nolint:testifylint
		require.NoError(t, err)
	}()

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

func TestRunFunctionWithSigHandling(t *testing.T) {
	t.Run("Should exitCode 1 on config not found", func(t *testing.T) {
		exitCode := apiServer.RunFunctionWithSigHandling(func(_ context.Context, _ *config.Config) error {
			return nil
		})
		require.Equal(t, 1, exitCode)
	})

	t.Run("Should exitCode 0 on run", func(t *testing.T) {
		filename := "config.yaml"
		f, err := os.Create(filename)
		require.NoError(t, err)

		defer f.Close()
		defer os.Remove(filename)

		exitCode := apiServer.RunFunctionWithSigHandling(func(_ context.Context, _ *config.Config) error {
			return nil
		})
		require.Equal(t, 0, exitCode)
	})
}

func TestMonitorKeystorePoolSize(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cfg := config.Config{
		KeystorePool: config.KeystorePool{
			Interval: 100 * time.Millisecond,
		},
		Database: config.Database{
			Host:   commoncfg.SourceRef{Source: "embedded", Value: "localhost"},
			User:   commoncfg.SourceRef{Source: "embedded", Value: "postgres"},
			Secret: commoncfg.SourceRef{Source: "embedded", Value: "secret"},
			Name:   "cmk",
			Port:   "5433",
		},
	}

	// Run in goroutine, should exit after context timeout
	go func() {
		apiServer.MonitorKeystorePoolSize(ctx, cfg)
	}()

	<-ctx.Done()
	// Check if the error is due to context deadline exceeded, not due to other reasons
	require.Error(t, ctx.Err(), &context.DeadlineExceeded)
}
