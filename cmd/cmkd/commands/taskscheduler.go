package commands

import (
	"context"
	"fmt"

	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	"github.tools.sap/kms/cmk/internal/async"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/log"
	runcmd "github.tools.sap/kms/cmk/utils/cmd"
	statusserver "github.tools.sap/kms/cmk/utils/status_server"
)

func NewTaskScheduler() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task-scheduler",
		Short: "Start the task scheduler",
		Long:  `Starts the cron-based task scheduler that enqueues periodic jobs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			exitCode := runcmd.RunFuncWithSignalHandling(runTaskScheduler, runcmd.RunFlags{
				GracefulShutdownSec:     gracefulShutdownSec,
				GracefulShutdownMessage: gracefulShutdownMessage,
				Env:                     constants.APIName + "_task_scheduler",
			})
			if exitCode != 0 {
				return fmt.Errorf("task-scheduler exited with code %d", exitCode)
			}
			return nil
		},
	}

	cmd.Flags().Int64Var(&gracefulShutdownSec, "graceful-shutdown", 1, "graceful shutdown seconds")
	cmd.Flags().StringVar(&gracefulShutdownMessage, "graceful-shutdown-message",
		"Graceful shutdown in %d seconds", "graceful shutdown message")

	return cmd
}

func runTaskScheduler(ctx context.Context, cfg *config.Config) error {
	err := logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("task-scheduler").Wrapf(err, "Failed to initialise the logger")
	}

	statusserver.StartStatusServer(ctx, cfg)

	cronJob, err := async.New(cfg)
	if err != nil {
		return oops.In("task-scheduler").Wrapf(err, "failed to create the scheduler")
	}

	err = cronJob.RunScheduler()
	if err != nil {
		return oops.In("task-scheduler").Wrapf(err, "failed to start the scheduler job")
	}

	<-ctx.Done()

	err = cronJob.Shutdown(ctx)
	if err != nil {
		return oops.In("task-scheduler").Wrapf(err, "failed to shutdown the scheduler")
	}

	log.Info(ctx, "shutting down scheduler")

	return nil
}
