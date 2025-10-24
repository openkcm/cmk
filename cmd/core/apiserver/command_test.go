package apiserver_test

import (
	"context"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/cmd/core/apiserver"
	"github.com/openkcm/cmk/internal/config"
)

func TestRun(t *testing.T) {
	t.Run("Should error on not possible database connection", func(t *testing.T) {
		err := apiserver.Run(t.Context(), &config.Config{
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
		apiserver.MonitorKeystorePoolSize(ctx, cfg)
	}()

	<-ctx.Done()
	// Check if the error is due to context deadline exceeded, not due to other reasons
	require.Error(t, ctx.Err(), &context.DeadlineExceeded)
}
