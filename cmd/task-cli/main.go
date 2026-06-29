package main

import (
	"context"
	"flag"
	"os"

	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/cmd/task-cli/commands"
	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	cliUtils "github.com/openkcm/cmk/utils/cli"
	"github.com/openkcm/cmk/utils/cmd"
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

	root := &cobra.Command{
		Use:   "task",
		Short: "Async Task CLI",
		Long:  "CLI tool to manage and invoke CMK asynchronous tasks.",
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			ctx := context.WithValue(ctx, commands.AsyncClientKey, asyncApp.Client())
			ctx = context.WithValue(ctx, commands.AsyncInspectorKey, asyncApp.Inspector())

			cmd.SetContext(ctx)
		},
	}

	root.AddCommand(commands.NewStatsCmd())
	root.AddCommand(commands.NewQueuesCmd())
	root.AddCommand(commands.NewInvokeCmd())
	root.AddCommand(cliUtils.NewSleep(cfg))

	err = root.ExecuteContext(ctx)
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
