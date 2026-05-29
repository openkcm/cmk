package main

import (
	"context"
	"errors"
	"flag"
	"os"

	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/utils/cmd"
	statusserver "github.com/openkcm/cmk/utils/status_server"
)

var (
	BuildInfo               = "{}"
	gracefulShutdownSec     = flag.Int64("graceful-shutdown", 1, "graceful shutdown seconds")
	gracefulShutdownMessage = flag.String("graceful-shutdown-message", "Graceful shutdown in %d seconds",
		"graceful shutdown message")
	ErrRegistryEnabled = errors.New("failed to create registry client")
)

const AppName = "scheduler"

func run(ctx context.Context, cfg *config.Config) error {
	err := logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("main").
			Wrapf(err, "Failed to initialise the logger")
	}

	statusserver.StartStatusServer(ctx, cfg)

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

	log.Info(ctx, "shutting down scheduler")

	return nil
}

func main() {
	flag.Parse()

	exitCode := cmd.RunFuncWithSignalHandling(run, cmd.RunFlags{
		GracefulShutdownSec:     *gracefulShutdownSec,
		GracefulShutdownMessage: *gracefulShutdownMessage,
		Env:                     constants.APIName + "_task_scheduler",
	})
	os.Exit(exitCode)
}
