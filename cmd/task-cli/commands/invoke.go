package commands

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	asyncUtils "github.com/openkcm/cmk/utils/async"
)

func NewInvokeCmd(ctx context.Context, asyncClient async.Client) *cobra.Command {
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
				taskInfo, err := asyncClient.Enqueue(task)
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

	cmd.SetContext(ctx)
	cmd.Flags().StringVar(&taskName, "task", "", "Task name to invoke")
	cmd.Flags().StringSliceVar(&tenants, "tenants", nil, "Comma-separated list of tenant IDs")

	err := cmd.MarkFlagRequired("task")
	if err != nil {
		cmd.PrintErrf("failed to mark flag 'task' as required: %v\n", err)
	}

	return cmd
}
