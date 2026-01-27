package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

// NewRootCmdWithInfinitySleep creates a new root cobra command with infinite sleep option.
// The command will sleep indefinitely when the --sleep flag is provided.
// This is useful for containers so the CLI can be invoked by exec commands while keeping the container running.
func NewRootCmdWithInfinitySleep(
	ctx context.Context,
	use string,
	shortDesc string,
	longDesc string,
) *cobra.Command {
	var sleep bool

	rootCmd := &cobra.Command{
		Use:   use,
		Short: shortDesc,
		Long:  longDesc,

		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},

		Run: func(cmd *cobra.Command, _ []string) {
			if sleep {
				infiniteRun(cmd)
			}
		},
	}

	rootCmd.PersistentFlags().BoolVar(&sleep, "sleep", false, "Enable sleep mode")
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
