package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"

	"github.com/openkcm/cmk/cmd/task-cli/commands"
	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/log"
)

func runFuncWithSignalHandling(f func(context.Context, *config.Config) error) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create signal channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Handle signals in a separate goroutine
	go func() {
		<-sigChan
		log.Info(ctx, "Interrupt signal received, shutting down...")
		cancel()
	}()

	cfg, err := config.LoadConfig(commoncfg.WithEnvOverride(constants.APIName + "_task_cli"))
	if err != nil {
		log.Error(ctx, "Failed to load config:", err)

		return 1
	}

	log.Debug(ctx, "Starting the application", slog.Any("config", cfg))

	err = f(ctx, cfg)
	if err != nil {
		log.Error(ctx, "Failed running task-cli", err)
		return 1
	}

	return 0
}

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

	rootCmd := commands.NewRootCmd(ctx)
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
	exitCode := runFuncWithSignalHandling(run)
	os.Exit(exitCode)
}
