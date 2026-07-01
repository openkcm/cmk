package commands

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/log"
	asyncUtils "github.com/openkcm/cmk/utils/async"
	cmkContext "github.com/openkcm/cmk/utils/context"
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

const (
	pageSize    = 10
	historyDays = 7
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

			ctx = context.WithValue(ctx, AsyncClientKey, client)
			ctx = context.WithValue(ctx, AsyncInspectorKey, inspector)
			cmd.SetContext(ctx)

			return nil
		},
	}

	cmd.AddCommand(NewStatsCmd())
	cmd.AddCommand(NewQueuesCmd())
	cmd.AddCommand(NewInvokeCmd())

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

func NewInvokeCmd() *cobra.Command {
	var (
		taskName string
		tenants  []string
	)

	cmd := &cobra.Command{
		Use:   "invoke",
		Short: "Invoke a scheduled task",
		Long: "Invoke a scheduled task immediately by providing its task name. \n" +
			"List of tenant IDs can be provided to invoke the task for specific tenants. \n" +
			"For example: task-cli invoke --task <task-name> --tenants <tenant1,tenant2>",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client := cmkContext.GetFromContext[async.Client](cmd.Context(), AsyncClientKey)

			switch taskName {
			case config.TypeCertificateTask, config.TypeSystemsTask, config.TypeHYOKSync,
				config.TypeWorkflowExpire, config.TypeWorkflowCleanup, config.TypeKeystorePool:
				var payload []byte
				if len(tenants) > 0 {
					p := asyncUtils.NewTenantListPayload(tenants)

					b, err := p.ToBytes()
					if err != nil {
						cmd.PrintErrf("Failed to create payload: %v", err)
						return err
					}

					payload = b
				}
				task := asynq.NewTask(taskName, payload)
				taskInfo, err := client.Enqueue(task)
				if err != nil {
					cmd.PrintErrf("Failed to enqueue task: %v", err)
					return err
				}
				cmd.Printf("Task %s enqueued with ID: %s\n", taskName, taskInfo.ID)
				return nil
			default:
				cmd.PrintErrf("Unknown task name or not supported: %s\n", taskName)
				return nil
			}
		},
	}

	cmd.Flags().StringVar(&taskName, "task", "", "Task name to invoke")
	cmd.Flags().StringSliceVar(&tenants, "tenants", nil, "Comma-separated list of tenant IDs")

	err := cmd.MarkFlagRequired("task")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'task' as required: %v\n", err)
	}

	return cmd
}

func NewQueuesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queues",
		Short: "List queues",
		Long:  "List queues in the Asynq task system",
		RunE: func(cmd *cobra.Command, _ []string) error {
			inspector := cmkContext.GetFromContext[Inspector](cmd.Context(), AsyncInspectorKey)

			queues, err := inspector.Queues()
			if err != nil {
				cmd.PrintErrf("Failed to list queues: %v", err)
				return err
			}

			cmd.Print("List of asynq queues:\n")
			for _, q := range queues {
				cmd.Printf("- %s\n", q)
			}

			return nil
		},
	}

	return cmd
}

//nolint:cyclop,funlen
func NewStatsCmd() *cobra.Command {
	var queue string
	var queueInfo, weeklyHistory, pendingTasks, activeTasks, completeTasks, archivedTasks bool
	var page int

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show Asynq task queue statistics",
		Long: "Show Asynq task queue statistics.\n" +
			"Specify the queue name and the type of statistics to display.\n" +
			"When displaying tasks, results are paginated with a default page size of 10.\n" +
			"Use the --page flag to navigate through pages.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			inspector := cmkContext.GetFromContext[Inspector](cmd.Context(), AsyncInspectorKey)

			var stats any
			switch {
			case queueInfo:
				s, err := inspector.GetQueueInfo(queue)
				if err != nil {
					cmd.PrintErrf("Failed to get queue info: %v", err)
					return err
				}
				stats = s
			case weeklyHistory:
				s, err := inspector.History(queue, historyDays)
				if err != nil {
					cmd.PrintErrf("Failed to get weekly history: %v", err)
					return err
				}
				stats = s
			case pendingTasks:
				s, err := inspector.ListPendingTasks(queue, asynq.PageSize(pageSize), asynq.Page(page))
				if err != nil {
					cmd.PrintErrf("Failed to get pending tasks: %v", err)
					return err
				}
				stats = s
			case activeTasks:
				s, err := inspector.ListActiveTasks(queue, asynq.PageSize(pageSize), asynq.Page(page))
				if err != nil {
					cmd.PrintErrf("Failed to get active tasks: %v", err)
					return err
				}
				stats = s
			case completeTasks:
				s, err := inspector.ListCompletedTasks(queue, asynq.PageSize(pageSize), asynq.Page(page))
				if err != nil {
					cmd.PrintErrf("Failed to get complete tasks: %v", err)
					return err
				}
				stats = s
			case archivedTasks:
				s, err := inspector.ListArchivedTasks(queue, asynq.PageSize(pageSize), asynq.Page(page))
				if err != nil {
					cmd.PrintErrf("Failed to get archived tasks: %v", err)
					return err
				}
				stats = s
			default:
				cmd.PrintErrf("No valid stats option selected")
				return nil
			}

			statsJSON, err := json.MarshalIndent(stats, "", "\t")
			if err != nil {
				cmd.PrintErrf("Failed to marshal stats to JSON: %v", err)
				return err
			}

			cmd.Print(string(statsJSON))
			cmd.Println()

			return nil
		},
	}

	cmd.Flags().StringVar(&queue, "queue", "", "Queue name")
	cmd.Flags().BoolVar(&queueInfo, "queue-info", false, "Show queue info")
	cmd.Flags().BoolVar(&weeklyHistory, "weekly-history", false, "Show weekly history")
	cmd.Flags().BoolVar(&pendingTasks, "pending-tasks", false, "Show pending tasks")
	cmd.Flags().BoolVar(&activeTasks, "active-tasks", false, "Show active tasks")
	cmd.Flags().BoolVar(&completeTasks, "complete-tasks", false, "Show complete tasks")
	cmd.Flags().BoolVar(&archivedTasks, "archived-tasks", false, "Show archived tasks")
	cmd.Flags().IntVar(&page, "page", 0, "Page number for paginated results")
	cmd.MarkFlagsMutuallyExclusive(
		"queue-info", "weekly-history", "pending-tasks", "active-tasks", "complete-tasks", "archived-tasks",
	)
	cmd.MarkFlagsOneRequired(
		"queue-info", "weekly-history", "pending-tasks", "active-tasks", "complete-tasks", "archived-tasks",
	)

	err := cmd.MarkFlagRequired("queue")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'queue' as required: %v\n", err)
	}

	return cmd
}
