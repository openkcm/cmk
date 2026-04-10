package model

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
)

// KeyVersion represents a version of a key in the database.
type KeyVersion struct {
	AutoTimeModel

	ID        uuid.UUID  `gorm:"type:uuid;primaryKey"`
	NativeID  string     `gorm:"type:varchar(255);not null"`
	KeyID     uuid.UUID  `gorm:"type:uuid;not null;index"`
	Key       Key        `gorm:"foreignKey:KeyID"`
	RotatedAt *time.Time `gorm:"type:timestamptz;not null"` // Rotation timestamp (latest = current version)
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
