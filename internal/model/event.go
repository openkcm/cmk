package model

import (
	"encoding/json"

	"github.com/openkcm/orbital"
)

// Event is a model that holds the result of the latest sent events
// that terminated unsuccessfully
type Event struct {
	AutoTimeModel

	Identifier string            `gorm:"type:varchar(255);primaryKey"`
	Type       string            `gorm:"type:varchar(255);not null"`
	Data       json.RawMessage   `gorm:"type:jsonb;not null"`
	Status     orbital.JobStatus `gorm:"type:varchar(255);not null"`
}

// TableName returns the table name for Key
func (Event) TableName() string {
	return "events"
}

func (Event) IsSharedModel() bool {
	return false
}
