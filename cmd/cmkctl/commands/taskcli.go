package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	taskcommands "github.tools.sap/kms/cmk/cmd/task-cli/commands"
	"github.tools.sap/kms/cmk/internal/async"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/log"
	runcmd "github.tools.sap/kms/cmk/utils/cmd"
)

func NewTaskCLI() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Async task management CLI",
		Long:  `Manage and invoke CMK asynchronous tasks, view queue statistics and task status.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			exitCode := runcmd.RunFuncWithSignalHandling(runTaskCLI, runcmd.RunFlags{
				GracefulShutdownSec:     gracefulShutdownSec,
				GracefulShutdownMessage: gracefulShutdownMessage,
				Env:                     constants.APIName + "_task_cli",
			})
			if exitCode != 0 {
				return fmt.Errorf("task-cli exited with code %d", exitCode)
			}
			return nil
		},
	}

	cmd.Flags().Int64Var(&gracefulShutdownSec, "graceful-shutdown", 1, "graceful shutdown seconds")
	cmd.Flags().StringVar(&gracefulShutdownMessage, "graceful-shutdown-message",
		"Graceful shutdown in %d seconds", "graceful shutdown message")

	return cmd
}

func runTaskCLI(ctx context.Context, cfg *config.Config) error {
	err := logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("task-cli").Wrapf(err, "Failed to initialise the logger")
	}

	log.Debug(ctx, "Starting task-cli", slog.Any("config", cfg))

	asyncApp, err := async.New(cfg)
	if err != nil {
		return oops.In("task-cli").Wrapf(err, "failed to create the async app")
	}

	asyncClient := asyncApp.Client()
	asyncInspector := asyncApp.Inspector()

	rootCmd := taskcommands.NewRootCmd(ctx, cfg)
	rootCmd.AddCommand(taskcommands.NewStatsCmd(ctx, asyncInspector))
	rootCmd.AddCommand(taskcommands.NewQueuesCmd(ctx, asyncInspector))
	rootCmd.AddCommand(taskcommands.NewInvokeCmd(ctx, asyncClient))

	err = rootCmd.ExecuteContext(ctx)
	if err != nil {
		return oops.In("task-cli").Wrapf(err, "error executing command")
	}

	return nil
}
