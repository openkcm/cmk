package model

import (
	"github.com/google/uuid"
)

const ResourceID = "resource_id"

type BaseLabel struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	Key        string    `gorm:"type:varchar(255);not null"`
	Value      string    `gorm:"type:varchar(255)"`
	ResourceID uuid.UUID `gorm:"type:uuid;not null"`
}

type KeyLabel struct {
	BaseLabel
	AutoTimeModel

	CryptoKey Key `gorm:"foreignKey:ResourceID"`
}

func (KeyLabel) TableName() string {
	return "key_labels"
}

func (KeyLabel) IsSharedModel() bool {
	return false
}
