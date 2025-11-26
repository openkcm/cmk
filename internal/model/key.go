package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
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

func (k *Key) GetManagementAccessData() map[string]any {
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

func (k *Key) GetCryptoAccessData() KeyAccessData {
	if k.CryptoAccessData == nil {
		return nil
	}

	var data KeyAccessData

	err := json.Unmarshal(k.CryptoAccessData, &data)
	if err != nil {
		return nil // Return nil if unmarshalling fails to avoid panic
	}

	return data
}

func (k *Key) SetCryptoAccessData(data KeyAccessData) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	k.CryptoAccessData = bytes

	return nil
}
