package model

import (
	"github.com/google/uuid"
)

// KeyConfiguration represents a key configuration in the database.
//
//nolint:recvcheck
type KeyConfiguration struct {
	AutoTimeModel

	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name         string    `gorm:"type:varchar(255);not null;unique"`
	Description  string    `gorm:"type:text"`
	AdminGroupID uuid.UUID `gorm:"type:uuid;not null"`
	AdminGroup   Group     `gorm:"foreignKey:AdminGroupID"`
	CreatorID    uuid.UUID `gorm:"type:uuid;not null"`
	CreatorName  string    `gorm:"type:varchar(255);not null"`
	PrimaryKeyID *uuid.UUID
	TotalKeys    int                   `gorm:"->;-:migration"`
	TotalSystems int                   `gorm:"->;-:migration"`
	Tags         []KeyConfigurationTag `gorm:"many2many:keyconfigurations_tags;constraint:OnDelete:CASCADE"`
}

// TableName returns the table name for KeyConfiguration
func (KeyConfiguration) TableName() string {
	return "key_configurations"
}

func (KeyConfiguration) IsSharedModel() bool {
	return false
}

func (kc *KeyConfiguration) SetID(id uuid.UUID) {
	kc.ID = id
}
