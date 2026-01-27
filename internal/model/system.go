package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
)

//nolint:recvcheck
type System struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`

	Identifier string `gorm:"type:varchar(255);not null;uniqueindex:region_sys,priority:2"`

	Region               string            `gorm:"type:varchar(50);not null;uniqueindex:region_sys,priority:1"`
	Type                 string            `gorm:"type:varchar(50);not null"`
	KeyConfigurationID   *uuid.UUID        `gorm:"type:uuid"`
	KeyConfigurationName *string           `gorm:"->;-:migration"`
	Properties           map[string]string `gorm:"-:all"`

	// Status can be 'CONNECTED', 'DISCONNECTED', 'FAILED', or 'PROCESSING'
	Status cmkapi.SystemStatus `gorm:"type:varchar(50);default:'DISCONNECTED'"`
}

// TableName returns the table name for System
func (System) TableName() string {
	return "systems"
}

func (System) IsSharedModel() bool {
	return false
}

// UpdateSystemProperties if they are set
// and returns a bool if any field was updated
func (s *System) UpdateSystemProperties(
	props map[string]string,
	cfg *config.System,
) bool {
	updated := false

	for k, v := range props {
		_, ok := cfg.OptionalProperties[k]
		if ok {
			if s.Properties == nil {
				s.Properties = make(map[string]string)
			}

			s.Properties[k] = v
			updated = true
		}
	}

	return updated
}

// AfterSave is ran before any creating/updating the system
// but before finishing the transaction
// If this step fails the transaction should be aborted
func (s *System) AfterSave(tx *gorm.DB) error {
	props := make([]*SystemProperty, 0, len(s.Properties))
	for k, v := range s.Properties {
		props = append(props, &SystemProperty{
			ID:    s.ID,
			Key:   k,
			Value: v,
		})
	}

	if len(props) > 0 {
		//nolint:unqueryvet
		return tx.Select("*").Save(props).Error
	}

	return nil
}

// BeforeDelete is ran before deleting the system
// but before finishing the transaction
// If this step fails the transaction should be aborted
func (s *System) BeforeDelete(tx *gorm.DB) error {
	// Delete all associated system properties
	return tx.Where("id = ?", s.ID).Delete(&SystemProperty{}).Error
}

type SystemProperty struct {
	ID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	Key   string    `gorm:"type:varchar(255);primaryKey"`
	Value string    `gorm:"type:varchar(255)"`
}

func (SystemProperty) TableName() string {
	return "systems_properties"
}

func (SystemProperty) IsSharedModel() bool {
	return false
}

type JoinSystem struct {
	System

	Key   string `gorm:"type:varchar(255);primaryKey"`
	Value string `gorm:"type:varchar(255)"`
}
