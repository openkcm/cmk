package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/log"
)

type RunFlags struct {
	GracefulShutdownSec     int64
	GracefulShutdownMessage string
	Env                     string
}

// RunFuncWithSignalHandling runs the given function with signal handling. When
// a CTRL-C is received, the context will be cancelled on which the function can
// act upon.
// It returns the exitCode
func RunFuncWithSignalHandling(f func(context.Context, *config.Config) error, runFlags RunFlags) int {
	ctx, cancelOnSignal := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGTERM,
	)
	defer cancelOnSignal()

	cfg, err := config.LoadConfig(
		commoncfg.WithEnvOverride(runFlags.Env),
	)
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
	_, _ = fmt.Fprintln(os.Stderr, fmt.Sprintf(runFlags.GracefulShutdownMessage, runFlags.GracefulShutdownSec))
	time.Sleep(time.Duration(runFlags.GracefulShutdownSec) * time.Second)

	return 0
}
