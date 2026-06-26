package commands

import (
	"context"
	"log/slog"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	taskcommands "github.tools.sap/kms/cmk/cmd/task-cli/commands"
	"github.tools.sap/kms/cmk/internal/async"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/log"
)

func NewTaskCLI() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Async task management CLI",
		Long:  `Manage and invoke CMK asynchronous tasks, view queue statistics and task status`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			client, inspector, err := newTaskCLI(ctx)
			if err != nil {
				return err
			}

			ctx = context.WithValue(ctx, taskcommands.AsyncClientKey, client)
			ctx = context.WithValue(ctx, taskcommands.AsyncInspectorKey, inspector)
			cmd.SetContext(ctx)

			return nil
		},
	}

	cmd.AddCommand(taskcommands.NewStatsCmd())
	cmd.AddCommand(taskcommands.NewQueuesCmd())
	cmd.AddCommand(taskcommands.NewInvokeCmd())

	return cmd
}

func newTaskCLI(ctx context.Context) (async.Client, taskcommands.Inspector, error) {
	cfg, err := config.LoadConfig(
		commoncfg.WithEnvOverride(constants.APIName + "_task_cli"),
	)
	if err != nil {
		log.Error(ctx, "Failed to load config:", err)
		return nil, nil, err
	}

	err = logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return nil, nil, oops.In("task-cli").Wrapf(err, "Failed to initialise the logger")
	}

	log.Debug(ctx, "Starting task-cli", slog.Any("config", cfg))

	asyncApp, err := async.New(cfg)
	if err != nil {
		return nil, nil, oops.In("task-cli").Wrapf(err, "failed to create the async app")
	}

	asyncClient := asyncApp.Client()
	asyncInspector := asyncApp.Inspector()

	return asyncClient, asyncInspector, nil
}
