package main_test

import (
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"

	eventReconciler "github.com/openkcm/cmk/cmd/event-reconciler"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestRun(t *testing.T) {
	_, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})
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

	testutils.TestBinStartup(t, cfg.Status.Address, func() error {
		return eventReconciler.Run(t.Context(), cfg)
	})
}
