package taskworker

import (
	"context"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	cmklog "github.com/openkcm/cmk/internal/log"
)

func Cmd(buildInfo string) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "task-worker",
		Short: "CMK Task Worker",
		Long:  "CMK Task Worker - A background service that processes tasks asynchronously.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			defaultValues := map[string]any{}
			cfg := &config.Config{}

			err := commoncfg.LoadConfig(
				cfg,
				defaultValues,
				constants.DefaultConfigPath1,
				constants.DefaultConfigPath2,
				".",
			)
			if err != nil {
				return oops.In("main").Wrapf(err, "failed to load the config")
			}

			// Update Version
			err = commoncfg.UpdateConfigVersion(&cfg.BaseConfig, buildInfo)
			if err != nil {
				return oops.In("main").
					Wrapf(err, "Failed to update the version configuration")
			}

			// LoggerConfig initialisation
			err = logger.InitAsDefault(cfg.Logger, cfg.Application)
			if err != nil {
				return oops.In("main").
					Wrapf(err, "Failed to initialise the logger")
			}

			cronJob, err := async.New(cfg)
			if err != nil {
				return oops.In("main").Wrapf(err, "failed to create the worker")
			}

			err = cronJob.RunWorker(ctx, cfg)
			if err != nil {
				return oops.In("main").Wrapf(err, "failed to start the worker")
			}

			<-ctx.Done()

			err = cronJob.Shutdown(ctx)
			if err != nil {
				return oops.In("main").Wrapf(err, "%s", async.ErrClientShutdown.Error())
			}

			cmklog.Info(ctx, "shutting down worker")

			return nil
		},
	}

	return cmd
}
