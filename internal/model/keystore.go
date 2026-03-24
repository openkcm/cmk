package model

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/config"
)

// Keystore is an internal entity of pool item that should be persisted.
type Keystore struct {
	AutoTimeModel

	ID       uuid.UUID       `gorm:"type:uuid;primaryKey"`
	Provider string          `gorm:"type:varchar(50);not null"`
	Config   json.RawMessage `gorm:"type:jsonb;not null;unique"`
}

// TableResourceType return the authz resource type
func (m Keystore) TableResourceType() authz.RepoResourceTypeName {
	return authz.RepoResourceTypeKeystore
}

func (m Keystore) TableName() string {
	return string(m.TableResourceType())
}

func (Keystore) IsSharedModel() bool {
	return true
}

func (m Keystore) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceTypeName, authz.RepoAction],
	action authz.RepoAction) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
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
