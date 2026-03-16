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

func createOrbitalRepository(ctx context.Context, cfg config.Database) (*orbital.Repository, error) {
	baseDSN, err := dsn.FromDBConfig(cfg)
	if err != nil {
		return nil, err
	}

	dsn := baseDSN + " search_path=orbital,public"

	con, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("orbit pool: %w", err)
	}
	store, err := orbsql.New(ctx, con)
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
