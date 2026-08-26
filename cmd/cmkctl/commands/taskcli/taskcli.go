package taskcli

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/cmd/cmkctl/commands/taskcli/commands"
	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/log"
)

type Inspector interface {
	Queues() ([]string, error)
	GetQueueInfo(queue string) (*asynq.QueueInfo, error)
	History(queue string, days int) ([]*asynq.DailyStats, error)
	ListPendingTasks(queue string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
	ListActiveTasks(queue string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
	ListCompletedTasks(queue string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
	ListArchivedTasks(queue string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
}

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

			ctx = context.WithValue(ctx, commands.AsyncClientKey, client)
			ctx = context.WithValue(ctx, commands.AsyncInspectorKey, inspector)
			cmd.SetContext(ctx)

			return nil
		},
	}

	cmd.AddCommand(commands.NewStatsCmd())
	cmd.AddCommand(commands.NewQueuesCmd())
	cmd.AddCommand(commands.NewInvokeCmd())

	return cmd
}

func newTaskCLI(ctx context.Context) (async.Client, Inspector, error) {
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
