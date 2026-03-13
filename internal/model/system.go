package model

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/authz"
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

	// Only set for failed systems by the event table
	ErrorCode    string `gorm:"->"`
	ErrorMessage string `gorm:"->"`
}

// TableResourceType return the authz resource type
func (m System) TableResourceType() authz.RepoResourceTypeName {
	return authz.RepoResourceTypeSystem
}

// TableName returns the table name for System
func (m System) TableName() string {
	return string(m.TableResourceType())
}

func (System) IsSharedModel() bool {
	return false
}

func (m System) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceTypeName, authz.RepoAction],
	action authz.RepoAction) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}

// UpdateSystemProperties if they are set
// and returns a bool if any field was updated
func (m *System) UpdateSystemProperties(
	props map[string]string,
	cfg *config.System,
) bool {
	updated := false

	for k, v := range props {
		_, ok := cfg.OptionalProperties[k]
		if ok {
			if m.Properties == nil {
				m.Properties = make(map[string]string)
			}

			m.Properties[k] = v
			updated = true
		}
	}

	return updated
}

// AfterSave is ran before any creating/updating the system
// but before finishing the transaction
// If this step fails the transaction should be aborted
func (m *System) AfterSave(tx *gorm.DB) error {
	props := make([]*SystemProperty, 0, len(m.Properties))
	for k, v := range m.Properties {
		props = append(props, &SystemProperty{
			ID:    m.ID,
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
func (m *System) BeforeDelete(tx *gorm.DB) error {
	// Delete all associated system properties
	return tx.Where("id = ?", m.ID).Delete(&SystemProperty{}).Error
}

type SystemProperty struct {
	ID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	Key   string    `gorm:"type:varchar(255);primaryKey"`
	Value string    `gorm:"type:varchar(255)"`
}

// TableResourceType return the authz resource type
func (m SystemProperty) TableResourceType() authz.RepoResourceTypeName {
	return authz.RepoResourceTypeSystemProperty
}

func (m SystemProperty) TableName() string {
	return string(m.TableResourceType())
}

func (SystemProperty) IsSharedModel() bool {
	return false
}

func (m SystemProperty) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceTypeName, authz.RepoAction],
	action authz.RepoAction) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}

type JoinSystem struct {
	System

	Key   string `gorm:"type:varchar(255);primaryKey"`
	Value string `gorm:"type:varchar(255)"`
}
