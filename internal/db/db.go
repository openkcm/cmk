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
	cfg *config.Config,
) (*multitenancy.DB, error) {
	log.Info(ctx, "Starting DB connection ")

	dbCon, err := StartDBConnection(ctx, cfg.Database, cfg.DatabaseReplicas)
	if err != nil {
		return nil, oops.In(DBLogDomain).Wrapf(err, "failed to initialize DB Connection")
	}

	err = addKeystoreFromConfig(ctx, dbCon, cfg.Provisioning.InitKeystoreConfig)
	if err != nil {
		return nil, oops.In(DBLogDomain).Wrapf(err, "failed to add initial keystore config")
	}

	return dbCon, nil
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

	ks := &model.Keystore{
		ID:       uuid.New(),
		Provider: initKeystoreConfig.Provider,
		Config:   json.RawMessage(valueBytes),
	}

	err = db.WithContext(ctx).Create(ks).Error
	if err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
		return errs.Wrapf(err, "failed to save keystore configuration")
	}

	return nil
}
