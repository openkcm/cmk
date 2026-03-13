package model

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
)

// ImportParams represents the parameters for a Bring Your Own Key (BYOK) configuration.
type ImportParams struct {
	AutoTimeModel

	KeyID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	WrappingAlg        string    `gorm:"type:varchar(50);not null"`
	HashFunction       string    `gorm:"type:varchar(50);not null"`
	PublicKeyPEM       string    `gorm:"type:text;not null"`
	Expires            *time.Time
	ProviderParameters json.RawMessage `gorm:"type:jsonb"`
}

// TableResourceType return the authz resource type
func (m ImportParams) TableResourceType() authz.RepoResourceTypeName {
	return authz.RepoResourceTypeImportparam
}

// TableName returns the table name for ImportParams
func (m ImportParams) TableName() string {
	return string(m.TableResourceType())
}

func (ImportParams) IsSharedModel() bool {
	return false
}

func (m ImportParams) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceTypeName, authz.RepoAction],
	action authz.RepoAction) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}

// IsExpired checks if the ImportParams has expired based on the Expires field.
func (b ImportParams) IsExpired() bool {
	if b.Expires == nil {
		return false
	}

	return time.Now().After(*b.Expires)
}
