package commands

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/config"
	statusserver "github.com/openkcm/cmk/utils/status_server"
)

func NewSleep() *cobra.Command {
	return &cobra.Command{
		Use:   "sleep",
		Short: "Sleep",
		Long:  "Sleep",

		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}

			statusserver.StartStatusServer(cmd.Context(), cfg)

			cmd.Println("Pod running...")

			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

			<-sigs
			cmd.Println("Shutting down gracefully...")

			return nil
		},
	}
}
