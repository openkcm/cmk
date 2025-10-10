package model

import "github.com/google/uuid"

type BaseTag struct {
	ID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	Value string    `gorm:"type:varchar(255);not null;unique"`
}

//nolint:recvcheck
type KeyConfigurationTag struct {
	BaseTag

	KeyConfigurations []KeyConfiguration `gorm:"many2many:keyconfigurations_tags;"`
}

// TableName returns the table name for Key
func (KeyConfigurationTag) TableName() string {
	return "key_configuration_tags"
}

func (KeyConfigurationTag) IsSharedModel() bool {
	return false
}

func (kct *KeyConfigurationTag) SetTag(tag BaseTag) {
	kct.BaseTag = tag
}
