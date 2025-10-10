package model

import (
	"github.com/google/uuid"
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

// TableName returns the table name for KeyVersion
func (KeyVersion) TableName() string {
	return "key_versions"
}

func (KeyVersion) IsSharedModel() bool {
	return false
}
