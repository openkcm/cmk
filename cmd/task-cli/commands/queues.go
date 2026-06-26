package commands

import (
	"github.com/spf13/cobra"

	"github.tools.sap/kms/cmk/utils/context"
)

func NewQueuesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queues",
		Short: "List queues",
		Long:  "List queues in the Asynq task system",
		RunE: func(cmd *cobra.Command, _ []string) error {
			inspector := context.GetFromContext[Inspector](cmd.Context(), AsyncInspectorKey)

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
