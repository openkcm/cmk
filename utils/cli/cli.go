package cli

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/config"
	statusserver "github.com/openkcm/cmk/utils/status_server"
)

func NewSleep(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "sleep",
		Short: "sleep",
		Long:  "sleep",
		Run: func(cmd *cobra.Command, args []string) {
			statusserver.StartStatusServer(cmd.Context(), cfg)

			cmd.Println("Pod running...")

			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

			<-sigs
			cmd.Println("Shutting down gracefully...")
		},
	}
}
