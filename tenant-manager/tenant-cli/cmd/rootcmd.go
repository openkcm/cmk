package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func (f *CommandFactory) NewRootCmd(ctx context.Context) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "tm",
		Short: "Tenant Manager CLI Application",
		Long: "Tenant Manager is a simple CLI tool to manage tenants, supporting: creating tenant, " +
			"creating tenant with groups, " +
			"creating groups, " +
			"updating of region and status field on a tenant entity in public table, " +
			"updating of group names, " +
			"changing any field value in any table of a tenant schema.",

		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},

		Run: func(cmd *cobra.Command, _ []string) {
			sleep, _ := cmd.Flags().GetBool("sleep")
			if sleep {
				infiniteRun(cmd)
			}
		},
	}

	rootCmd.SetContext(ctx)

	return rootCmd
}

func infiniteRun(cmd *cobra.Command) {
	cmd.Println("Pod running...")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs
	cmd.Println("Shutting down gracefully...")
}
