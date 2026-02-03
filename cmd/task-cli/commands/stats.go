package commands

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	"github.com/spf13/cobra"
)

const (
	pageSize    = 10
	historyDays = 7
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

//nolint:cyclop,funlen
func NewStatsCmd(ctx context.Context, asyncInspector Inspector) *cobra.Command {
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
			var stats any
			switch {
			case queueInfo:
				s, err := asyncInspector.GetQueueInfo(queue)
				if err != nil {
					cmd.PrintErrf("Failed to get queue info: %v", err)
					return err
				}
				stats = s
			case weeklyHistory:
				s, err := asyncInspector.History(queue, historyDays)
				if err != nil {
					cmd.PrintErrf("Failed to get weekly history: %v", err)
					return err
				}
				stats = s
			case pendingTasks:
				s, err := asyncInspector.ListPendingTasks(queue, asynq.PageSize(pageSize), asynq.Page(page))
				if err != nil {
					cmd.PrintErrf("Failed to get pending tasks: %v", err)
					return err
				}
				stats = s
			case activeTasks:
				s, err := asyncInspector.ListActiveTasks(queue, asynq.PageSize(pageSize), asynq.Page(page))
				if err != nil {
					cmd.PrintErrf("Failed to get active tasks: %v", err)
					return err
				}
				stats = s
			case completeTasks:
				s, err := asyncInspector.ListCompletedTasks(queue, asynq.PageSize(pageSize), asynq.Page(page))
				if err != nil {
					cmd.PrintErrf("Failed to get complete tasks: %v", err)
					return err
				}
				stats = s
			case archivedTasks:
				s, err := asyncInspector.ListArchivedTasks(queue, asynq.PageSize(pageSize), asynq.Page(page))
				if err != nil {
					cmd.PrintErrf("Failed to get archived tasks: %v", err)
					return err
				}
				stats = s
			default:
				cmd.PrintErrf("No valid stats option selected")
				return nil
			}

			statsJson, err := json.MarshalIndent(stats, "", "\t")
			if err != nil {
				cmd.PrintErrf("Failed to marshal stats to JSON: %v", err)
				return err
			}

			cmd.Print(string(statsJson))
			cmd.Println()

			return nil
		},
	}

	cmd.SetContext(ctx)
	cmd.Flags().StringVar(&queue, "queue", "", "Queue name")
	cmd.Flags().BoolVar(&queueInfo, "queue-info", false, "Show queue info")
	cmd.Flags().BoolVar(&weeklyHistory, "weekly-history", false, "Show weekly history")
	cmd.Flags().BoolVar(&pendingTasks, "pending-tasks", false, "Show pending tasks")
	cmd.Flags().BoolVar(&activeTasks, "active-tasks", false, "Show active tasks")
	cmd.Flags().BoolVar(&completeTasks, "complete-tasks", false, "Show complete tasks")
	cmd.Flags().BoolVar(&archivedTasks, "archived-tasks", false, "Show archived tasks")
	cmd.Flags().IntVar(&page, "page", 0, "Page number for paginated results")
	cmd.MarkFlagsMutuallyExclusive(
		"queue-info", "weekly-history", "pending-tasks", "active-tasks", "complete-tasks", "archived-tasks")
	cmd.MarkFlagsOneRequired(
		"queue-info", "weekly-history", "pending-tasks", "active-tasks", "complete-tasks", "archived-tasks")

	err := cmd.MarkFlagRequired("queue")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'queue' as required: %v\n", err)
	}

	return cmd
}
