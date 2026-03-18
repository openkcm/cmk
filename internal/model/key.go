package model

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/authz"
)

type KeyAccessData map[string]map[string]any // Map of regions and their properties

//nolint:recvcheck
type Key struct {
	AutoTimeModel

	ID                   uuid.UUID     `gorm:"type:uuid;primaryKey"`
	KeyConfigurationID   uuid.UUID     `gorm:"type:uuid;not null;uniqueindex:keyname,priority:1"`
	Name                 string        `gorm:"type:varchar(255);not null;uniqueindex:keyname,priority:2"`
	KeyType              string        `gorm:"type:varchar(50);not null"`
	Description          string        `gorm:"type:text"`
	Algorithm            string        `gorm:"type:varchar(50);not null"`
	Provider             string        `gorm:"type:varchar(50);not null"`
	Region               string        `gorm:"type:varchar(50);not null"`
	State                string        `gorm:"type:varchar(50);not null;default:'ENABLED'"`
	KeyVersions          []KeyVersion  `gorm:"foreignKey:KeyID"`
	ImportParams         *ImportParams `gorm:"foreignKey:KeyID;references:ID;constraint:OnDelete:CASCADE"`
	NativeID             *string       `gorm:"type:varchar(255)"`
	KeyLabels            []KeyLabel    `gorm:"foreignKey:ResourceID"`
	LastUsed             *time.Time
	ManagementAccessData json.RawMessage `gorm:"type:jsonb"`
	CryptoAccessData     json.RawMessage `gorm:"type:jsonb"`
	EditableRegions      map[string]bool `gorm:"-:all"`
}

// TableResourceType return the authz resource type
func (m Key) TableResourceType() authz.RepoResourceTypeName {
	return authz.RepoResourceTypeKey
}

// TableName returns the table name for Key
func (m Key) TableName() string {
	return string(m.TableResourceType())
}

func (Key) IsSharedModel() bool {
	return false
}

func (m Key) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceTypeName, authz.RepoAction],
	action authz.RepoAction,
) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}

func (m *Key) AfterFind(tx *gorm.DB) error {
	m.EditableRegions = map[string]bool{}
	return nil
}

func (m *Key) GetManagementAccessData() map[string]any {
	if m.ManagementAccessData == nil {
		return nil
	}

	var data map[string]any

	err := json.Unmarshal(m.ManagementAccessData, &data)
	if err != nil {
		return nil // Return nil if unmarshalling fails to avoid panic
	}

	return data
}

func (m *Key) GetCryptoAccessData() KeyAccessData {
	if m.CryptoAccessData == nil {
		return nil
	}

	var data KeyAccessData

	err := json.Unmarshal(m.CryptoAccessData, &data)
	if err != nil {
		return nil // Return nil if unmarshalling fails to avoid panic
	}

	return data
}
