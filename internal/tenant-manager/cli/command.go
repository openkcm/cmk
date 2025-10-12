package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/db"
)

var (
	id, region, status string
	sleep              bool
)

type CommandFactory struct {
	dbConn *multitenancy.DB
}

func NewCommandFactory() *CommandFactory {
	return &CommandFactory{}
}

func NewCommandFactoryWithDB(dbConn *multitenancy.DB) *CommandFactory {
	return &CommandFactory{
		dbConn: dbConn,
	}
}

func loadConfig() (*config.Config, error) {
	cfg := &config.Config{}

	loader := commoncfg.NewLoader(
		cfg,
		commoncfg.WithPaths(
			constants.DefaultConfigPath1,
			constants.DefaultConfigPath2,
			".",
		),
		commoncfg.WithEnvOverride(constants.APIName),
	)

	err := loader.LoadConfig()
	if err != nil {
		return nil, oops.Hint("failed to load config").Wrap(err)
	}

	err = logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return nil, oops.Hint("Unable to initialize logger").Wrap(err)
	}

	return cfg, nil
}

func (f *CommandFactory) db(ctx context.Context) (*multitenancy.DB, error) {
	if f.dbConn != nil {
		return f.dbConn, nil
	}

	cfg, err := loadConfig()
	if err != nil {
		return nil, oops.Hint("failed to load config").Wrap(err)
	}

	dbCon, err := db.StartDB(ctx, cfg.Database, cfg.Provisioning, cfg.DatabaseReplicas)
	if err != nil {
		return nil, oops.Hint("Check database configuration").Wrap(err)
	}

	f.dbConn = dbCon

	return f.dbConn, err
}

func InitWithCommandFactory(cmds ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cli",
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
				cmd.Println("Pod running...")

				sigs := make(chan os.Signal, 1)
				signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

				<-sigs
				cmd.Println("Shutting down gracefully...")
			}
		},
	}

	cmd.PersistentFlags().BoolVar(&sleep, "sleep", false, "Enable sleep mode")

	cmd.AddCommand(cmds...)

	return cmd
}
