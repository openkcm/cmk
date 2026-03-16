package eventprocessor

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/openkcm/orbital"

	orbsql "github.com/openkcm/orbital/store/sql"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db/dsn"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
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

func GetSystemJobData(e *model.Event) (SystemActionJobData, error) {
	var jobData SystemActionJobData
	return jobData, json.Unmarshal(e.Data, &jobData)
}

func unmarshalKeyJobData(job orbital.Job) (KeyActionJobData, error) {
	var data KeyActionJobData

	err := json.Unmarshal(job.Data, &data)
	if err != nil {
		return KeyActionJobData{}, fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	return data, nil
}

func unmarshalSystemJobData(job orbital.Job) (SystemActionJobData, error) {
	var systemJobData SystemActionJobData

	err := json.Unmarshal(job.Data, &systemJobData)
	if err != nil {
		return SystemActionJobData{}, fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	return systemJobData, nil
}
