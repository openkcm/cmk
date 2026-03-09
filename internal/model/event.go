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

	// Stores error content for failed events
	// It's split from orbital error_message by ERROR_CODE:ErrorMessage
	ErrorCode    string `gorm:"type:varchar(255)"`
	ErrorMessage string `gorm:"type:varchar(255)"`

	// PreviousItemStatus represents the state an item was before the event was sent
	// This is used for cancel actions to recover an item to it's previous state
	PreviousItemStatus string `gorm:"type:varchar(255)"`
}

// TableName returns the table name for Key
func (Event) TableName() string {
	return "events"
}

func (Event) IsSharedModel() bool {
	return false
}
