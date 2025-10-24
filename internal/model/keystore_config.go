package model

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/openkcm/cmk-core/internal/config"
)

// KeystoreConfiguration is an internal entity of pool item that should be persisted.
type KeystoreConfiguration struct {
	AutoTimeModel

	ID       uuid.UUID       `gorm:"type:uuid;primaryKey"`
	Provider string          `gorm:"type:varchar(50);not null"`
	Value    json.RawMessage `gorm:"type:jsonb;not null;unique"`
}

// TableName of KeystoreConfiguration type.
func (KeystoreConfiguration) TableName() string {
	return "public.keystore_configurations"
}

func (KeystoreConfiguration) IsSharedModel() bool {
	return true
}

// DefaultKeystore represents keystore configuration
//
//nolint:tagliatelle
type DefaultKeystore struct {
	LocalityID           string             `yaml:"localityId" json:"localityId"`
	CommonName           string             `yaml:"commonName" json:"commonName"`
	ManagementAccessData KeystoreAccessData `yaml:"managementAccessData" json:"managementAccessData"`
	SupportedRegions     []config.Region    `yaml:"supportedRegions" json:"supportedRegions"`
	allowBYOK            bool               //nolint:unused
}

type KeystoreAccessData map[string]any
