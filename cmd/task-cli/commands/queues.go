package commands

import (
	"context"

	"github.com/spf13/cobra"
)

func NewQueuesCmd(_ context.Context, asyncInspector Inspector) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queues",
		Short: "List queues",
		Long:  "List queues in the Asynq task system",
		RunE: func(cmd *cobra.Command, _ []string) error {
			queues, err := asyncInspector.Queues()
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
