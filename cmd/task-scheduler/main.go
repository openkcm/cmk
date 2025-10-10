package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	cmklog "github.com/openkcm/cmk/internal/log"
)

const AppName = "scheduler"

func start() error {
	ctx, cancelOnSignal := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGTERM,
	)

	defer cancelOnSignal()

	defaultValues := map[string]any{}
	cfg := &config.Config{}

	err := commoncfg.LoadConfig(
		cfg,
		defaultValues,
		constants.DefaultConfigPath1,
		constants.DefaultConfigPath2,
		".",
	)
	if err != nil {
		return oops.In("main").Wrapf(err, "failed to load the config")
	}

	// LoggerConfig initialisation
	err = logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("main").
			Wrapf(err, "Failed to initialise the logger")
	}

	cronJob, err := async.New(cfg)
	if err != nil {
		return oops.In("main").Wrapf(err, "failed to create the scheduler")
	}

	err = cronJob.RunScheduler()
	if err != nil {
		return oops.In("main").Wrapf(err, "failed to start the scheduler job")
	}

	<-ctx.Done()

	err = cronJob.Shutdown(ctx)
	if err != nil {
		return oops.In("main").Wrapf(err, "failed to shutdown the scheduler")
	}

	cmklog.Info(ctx, "shutting down scheduler")

	return nil
}

func main() {
	err := start()
	if err != nil {
		log.Fatal(err)
	}
}
