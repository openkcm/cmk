package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Key represents a key in the database.
type Key struct {
	AutoTimeModel

	ID                   uuid.UUID     `gorm:"type:uuid;primaryKey"`
	Name                 string        `gorm:"type:varchar(255);not null;unique"`
	KeyType              string        `gorm:"type:varchar(50);not null"`
	Description          string        `gorm:"type:text"`
	Algorithm            string        `gorm:"type:varchar(50);not null"`
	Provider             string        `gorm:"type:varchar(50);not null"`
	Region               string        `gorm:"type:varchar(50);not null"`
	State                string        `gorm:"type:varchar(50);not null;default:'ENABLED'"`
	KeyVersions          []KeyVersion  `gorm:"foreignKey:KeyID"`
	ImportParams         *ImportParams `gorm:"foreignKey:KeyID;references:ID;constraint:OnDelete:CASCADE"`
	NativeID             *string       `gorm:"type:varchar(255)"`
	KeyConfigurationID   uuid.UUID     `gorm:"type:uuid;not null; index"`
	KeyLabels            []KeyLabel    `gorm:"foreignKey:ResourceID"`
	LastUsed             *time.Time
	ManagementAccessData json.RawMessage `gorm:"type:jsonb"`
	CryptoAccessData     json.RawMessage `gorm:"type:jsonb"`
	IsPrimary            bool            `gorm:"type:bool"`
}

// TableName returns the table name for Key
func (Key) TableName() string {
	return "keys"
}

func (Key) IsSharedModel() bool {
	return false
}

func (k Key) MaxVersion() int {
	maxVer := 0
	for _, kv := range k.KeyVersions {
		if kv.Version > maxVer {
			maxVer = kv.Version
		}
	}

	return maxVer
}

func (k Key) Version() *KeyVersion {
	maxVer := 0

	var keyVersion KeyVersion

	for _, kv := range k.KeyVersions {
		if kv.Version > maxVer {
			maxVer = kv.Version
			keyVersion = kv
		}
	}

	return &keyVersion
}

func (k Key) GetManagementAccessData() map[string]any {
	if k.ManagementAccessData == nil {
		return nil
	}

	var data map[string]any

	err := json.Unmarshal(k.ManagementAccessData, &data)
	if err != nil {
		return nil // Return nil if unmarshalling fails to avoid panic
	}

	return data
}

func (k Key) GetCryptoAccessData() map[string]any {
	if k.CryptoAccessData == nil {
		return nil
	}

	var data map[string]any

	err := json.Unmarshal(k.CryptoAccessData, &data)
	if err != nil {
		return nil // Return nil if unmarshalling fails to avoid panic
	}

	return data
}
