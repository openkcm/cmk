package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/log"
	cmklog "github.com/openkcm/cmk/internal/log"
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

	cmklog.Info(ctx, "shutting down scheduler")

	return nil
}

// runFuncWithSignalHandling runs the given function with signal handling. When
// a CTRL-C is received, the context will be cancelled on which the function can
// act upon.
// It returns the exitCode
func runFuncWithSignalHandling(f func(context.Context, *config.Config) error) int {
	ctx, cancelOnSignal := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGTERM,
	)
	defer cancelOnSignal()

	cfg, err := config.LoadConfig(commoncfg.WithEnvOverride(constants.APIName + "_task_scheduler"))
	if err != nil {
		log.Error(ctx, "Failed to load the configuration", err)
		_, _ = fmt.Fprintln(os.Stderr, err)

		return 1
	}

	log.Debug(ctx, "Starting the application", slog.Any("config", *cfg))

	err = f(ctx, cfg)
	if err != nil {
		log.Error(ctx, "Failed to start the application", err)
		_, _ = fmt.Fprintln(os.Stderr, err)

		return 1
	}

	// graceful shutdown so running goroutines may finish
	_, _ = fmt.Fprintln(os.Stderr, fmt.Sprintf(*gracefulShutdownMessage, *gracefulShutdownSec))
	time.Sleep(time.Duration(*gracefulShutdownSec) * time.Second)

	return 0
}

func main() {
	flag.Parse()

	exitCode := runFuncWithSignalHandling(run)
	os.Exit(exitCode)
}
