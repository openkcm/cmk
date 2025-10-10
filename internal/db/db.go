package db

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/samber/oops"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
)

var (
	ErrMigrationFailed           = errors.New("migration failed")
	ErrEmptyManagementAccessData = errors.New("management access data cannot be empty")
	ErrManagementAccessDataType  = errors.New("management access data must be a literal string")
	ErrNilManagementAccessData   = errors.New("management access data cannot be nil")
	ErrEmptyLocalityID           = errors.New("locality ID cannot be empty")
	ErrEmptyCommonName           = errors.New("common name cannot be empty")
	ErrEmptySupportedRegions     = errors.New("supported regions cannot be empty")
	ErrEmptyRegionName           = errors.New("region name cannot be empty")
	ErrEmptyRegionTechName       = errors.New("region technical name cannot be empty")
)

const DBLogDomain = "db"

// StartDB starts DB connection and runs migrations
func StartDB(
	ctx context.Context,
	dbConf config.Database,
	provisioning config.Provisioning,
	replicas []config.Database,
) (*multitenancy.DB, error) {
	log.Info(ctx, "Starting DB connection ")

	dbCon, err := StartDBConnection(dbConf, replicas)
	if err != nil {
		return nil, oops.In(DBLogDomain).Wrapf(err, "failed to initialize DB Connection")
	}

	dbCon = dbCon.WithContext(ctx)

	log.Info(ctx, "Starting DB migration")

	err = migrate(ctx, dbCon)
	if err != nil {
		return nil, oops.In(DBLogDomain).Wrapf(err, "failed to run table creation migration")
	}

	log.Info(ctx, "DB migration finished")

	err = addKeystoreFromConfig(ctx, dbCon, provisioning.InitKeystoreConfig)
	if err != nil {
		return nil, oops.In(DBLogDomain).Wrapf(err, "failed to add initial keystore config")
	}

	return dbCon, nil
}

// migrate runs DB migrations
func migrate(ctx context.Context, db *multitenancy.DB) error {
	err := db.RegisterModels(
		ctx,
		&model.KeyConfiguration{},
		&model.Key{},
		&model.KeyVersion{},
		&model.KeyLabel{},
		&model.System{},
		&model.SystemProperty{},
		&model.Workflow{},
		&model.WorkflowApprover{},
		&model.Tenant{},
		&model.TenantConfig{},
		&model.Certificate{},
		&model.Group{},
		&model.ImportParams{},
		&model.KeystoreConfiguration{},
	)
	if err != nil {
		return errs.Wrap(ErrMigrationFailed, err)
	}

	err = db.MigrateSharedModels(ctx)
	if err != nil {
		return errs.Wrap(ErrMigrationFailed, err)
	}

	return nil
}

func validateKeystoreConfig(initKeystoreConfig config.InitKeystoreConfig) (map[string]string, error) {
	if initKeystoreConfig.Value.LocalityID == "" {
		return nil, ErrEmptyLocalityID
	}

	if initKeystoreConfig.Value.CommonName == "" {
		return nil, ErrEmptyCommonName
	}

	err := validateSupportedRegions(initKeystoreConfig.Value.SupportedRegions)
	if err != nil {
		return nil, err
	}

	if initKeystoreConfig.Value.ManagementAccessData == nil {
		return nil, ErrNilManagementAccessData
	}

	managementAccessDataStr, ok := initKeystoreConfig.Value.ManagementAccessData.(string)
	if !ok {
		return nil, ErrManagementAccessDataType
	}

	var parsedAccessData map[string]string

	err = yaml.Unmarshal([]byte(managementAccessDataStr), &parsedAccessData)
	if err != nil {
		return nil, errs.Wrapf(err, "failed to unmarshal YAML config value")
	}

	if len(parsedAccessData) == 0 {
		return nil, ErrEmptyManagementAccessData
	}

	return parsedAccessData, nil
}

func validateSupportedRegions(regions []config.Region) error {
	if len(regions) == 0 {
		return ErrEmptySupportedRegions
	}

	for _, region := range regions {
		if region.Name == "" {
			return ErrEmptyRegionName
		}

		if region.TechnicalName == "" {
			return ErrEmptyRegionTechName
		}
	}

	return nil
}

func addKeystoreFromConfig(
	ctx context.Context,
	db *multitenancy.DB,
	initKeystoreConfig config.InitKeystoreConfig,
) error {
	if !initKeystoreConfig.Enabled {
		log.Info(ctx, "Initial keystore config will not be added to pool")
		return nil
	}

	parsedAccessData, err := validateKeystoreConfig(initKeystoreConfig)
	if err != nil {
		return err
	}

	combinedValue := config.KeystoreConfigValue{
		LocalityID:           initKeystoreConfig.Value.LocalityID,
		CommonName:           initKeystoreConfig.Value.CommonName,
		ManagementAccessData: parsedAccessData,
		SupportedRegions:     initKeystoreConfig.Value.SupportedRegions,
	}

	valueBytes, err := json.Marshal(combinedValue)
	if err != nil {
		return errs.Wrapf(err, "failed to marshal combined config to JSON")
	}

	ksConfig := &model.KeystoreConfiguration{
		ID:       uuid.New(),
		Provider: initKeystoreConfig.Provider,
		Value:    json.RawMessage(valueBytes),
	}

	err = db.WithContext(ctx).Create(ksConfig).Error
	if err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
		return errs.Wrapf(err, "failed to save keystore configuration")
	}

	return nil
}
