package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

type Tag struct {
	ID     uuid.UUID       `gorm:"type:uuid;primaryKey"` // ID of the Item
	Values json.RawMessage `gorm:"type:jsonb"`
}

func (Tag) TableName() string {
	return "tags"
}

func (Tag) IsSharedModel() bool {
	return false
}
