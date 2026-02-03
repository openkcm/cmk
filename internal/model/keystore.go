package model

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/config"
)

// Keystore is an internal entity of pool item that should be persisted.
type Keystore struct {
	AutoTimeModel

	ID       uuid.UUID       `gorm:"type:uuid;primaryKey"`
	Provider string          `gorm:"type:varchar(50);not null"`
	Config   json.RawMessage `gorm:"type:jsonb;not null;unique"`
}

func (Keystore) TableName() string {
	return "public.keystore_pool"
}

func (Keystore) IsSharedModel() bool {
	return true
}

//nolint:tagliatelle
type KeystoreConfig struct {
	LocalityID           string             `yaml:"localityId" json:"localityId"`
	CommonName           string             `yaml:"commonName" json:"commonName"`
	ManagementAccessData KeystoreAccessData `yaml:"managementAccessData" json:"managementAccessData"`
	SupportedRegions     []config.Region    `yaml:"supportedRegions" json:"supportedRegions"`
	allowBYOK            bool               //nolint:unused
}

type KeystoreAccessData map[string]any
