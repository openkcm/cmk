package statusserver

import (
	"context"
	"syscall"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/db/dsn"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/common-sdk/pkg/health"
	"github.com/openkcm/common-sdk/pkg/status"
)

func StartStatusServer(ctx context.Context, cfg *config.Config, opts ...health.Option) {
	dsnFromConfig, err := dsn.FromDBConfig(cfg.Database)
	if err != nil {
		log.Error(ctx, "Could not load DSN from database config", err)
	}

	healthOptions := append([]health.Option{
		health.WithDatabaseChecker(
			constants.DBDriver,
			dsnFromConfig,
		),
	}, opts...)

	go func() {
		err := status.Serve(ctx, &cfg.BaseConfig, healthOptions...)
		if err != nil {
			log.Error(ctx, "Failure on the status server", err)

			_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		}
	}()
}
