package eventprocessor

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/openkcm/orbital"

	orbsql "github.com/openkcm/orbital/store/sql"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db/dsn"
	"github.com/openkcm/cmk/internal/errs"
)

func initOrbitalSchema(ctx context.Context, dbCfg config.Database) (*sql.DB, error) {
	baseDSN, err := dsn.FromDBConfig(dbCfg)
	if err != nil {
		return nil, err
	}

	orbitalDSN := baseDSN + " search_path=orbital,public sslmode=disable"

	orbitalDB, err := sql.Open("postgres", orbitalDSN)
	if err != nil {
		return nil, fmt.Errorf("orbit pool: %w", err)
	}

	_, err = orbitalDB.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS orbital")
	if err != nil {
		return nil, fmt.Errorf("ensure schema: %w", err)
	}

	return orbitalDB, nil
}

func createOrbitalRepository(ctx context.Context, dbCfg config.Database) (*orbital.Repository, error) {
	orbitalDB, err := initOrbitalSchema(ctx, dbCfg)
	if err != nil {
		return nil, fmt.Errorf("init orbital schema: %w", err)
	}

	store, err := orbsql.New(ctx, orbitalDB)
	if err != nil {
		return nil, errs.Wrapf(err, "failed to create orbital store")
	}

	return orbital.NewRepository(store), nil
}
