package taskscheduler

import (
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	"github.com/openkcm/cmk-core/internal/async"
	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/constants"
	cmklog "github.com/openkcm/cmk-core/internal/log"
)

func Cmd(buildInfo string) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "task-scheduler",
		Short: "CMK Task Scheduler",
		Long:  "CMK Task Scheduler - Customizable and efficient task scheduling solution.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

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
				return oops.In("main").Wrapf(err, "failed to create the scheduler")
			}

			err = cronJob.RunScheduler()
			if err != nil {
				return oops.In("main").Wrapf(err, "failed to start the scheduler job")
			}

			<-ctx.Done()

			err = cronJob.Shutdown(ctx)
			if err != nil {
				return oops.In("main").Wrapf(err, "failed to shutdown the scheduler")
			}

			cmklog.Info(ctx, "shutting down scheduler")

			return err
		},
	}

	return cmd
}
