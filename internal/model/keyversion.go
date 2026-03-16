package model

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
)

// KeyVersion represents a version of a key in the database.
type KeyVersion struct {
	AutoTimeModel

	ExternalID string    `gorm:"type:varchar(255);primaryKey"`
	NativeID   *string   `gorm:"type:varchar(255)"`
	KeyID      uuid.UUID `gorm:"type:uuid;not null;uniqueindex:key_version,priority:1"`
	Key        Key       `gorm:"foreignKey:KeyID;association_foreignkey:ID"`
	Version    int       `gorm:"not null;default:0;uniqueindex:key_version,priority:2"`
	IsPrimary  bool      `gorm:"not null;default:false"`
}

// TableResourceType return the authz resource type
func (m KeyVersion) TableResourceType() authz.RepoResourceTypeName {
	return authz.RepoResourceTypeKeyversion
}

// TableName returns the table name for KeyVersion
func (m KeyVersion) TableName() string {
	return string(m.TableResourceType())
}

func (KeyVersion) IsSharedModel() bool {
	return false
}

func (m KeyVersion) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceTypeName, authz.RepoAction],
	action authz.RepoAction) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}
