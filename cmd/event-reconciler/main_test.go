package main_test

import (
	"context"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	eventReconciler "github.com/openkcm/cmk/cmd/event-reconciler"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestRun(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	_, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{WithOrbital: true})
	queueURL := testutils.StartRabbitMQ(t)

	cfg := &config.Config{
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
		Database: dbCfg,
		EventProcessor: config.EventProcessor{
			Targets: []config.Target{
				{
					Region: "eu1",
					AMQP: config.AMQP{
						URL:    queueURL,
						Target: "test-queue",
						Source: "test-exchange",
					},
				},
			},
		},
	}
	err := eventReconciler.Run(ctx, cfg)

	assert.NoError(t, err)
}
