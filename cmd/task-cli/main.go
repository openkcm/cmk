package main

import (
	"context"
	"flag"
	"os"

	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"

	"github.com/openkcm/cmk/cmd/task-cli/commands"
	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/log"
)

var (
	gracefulShutdownSec     = flag.Int64("graceful-shutdown", 1, "graceful shutdown seconds")
	gracefulShutdownMessage = flag.String("graceful-shutdown-message", "Graceful shutdown in %d seconds",
		"graceful shutdown message")
)

func run(ctx context.Context, cfg *config.Config) error {
	err := logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("main").Wrapf(err, "Failed to initialise the logger")
	}

	asyncApp, err := async.New(cfg)
	if err != nil {
		return oops.In("main").Wrapf(err, "failed to create the async app")
	}

	asyncClient := asyncApp.Client()
	asyncInspector := asyncApp.Inspector()

	rootCmd := commands.NewRootCmd(ctx, cfg)
	rootCmd.AddCommand(commands.NewStatsCmd(ctx, asyncInspector))
	rootCmd.AddCommand(commands.NewQueuesCmd(ctx, asyncInspector))
	rootCmd.AddCommand(commands.NewInvokeCmd(ctx, asyncClient))

	err = rootCmd.ExecuteContext(ctx)
	if err != nil {
		return oops.In("main").Wrapf(err, "error executing command")
	}

	return nil
}

func main() {
	flag.Parse()

	exitCode := cmd.RunFuncWithSignalHandling(run, cmd.RunFlags{
		GracefulShutdownSec:     *gracefulShutdownSec,
		GracefulShutdownMessage: *gracefulShutdownMessage,
		Env:                     constants.APIName + "_task_cli",
	})
	os.Exit(exitCode)
}
