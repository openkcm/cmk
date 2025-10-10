package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ImportParams represents the parameters for a Bring Your Own Key (BYOK) configuration.
type ImportParams struct {
	AutoTimeModel

	KeyID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	WrappingAlg        string    `gorm:"type:varchar(50);not null"`
	HashFunction       string    `gorm:"type:varchar(50);not null"`
	PublicKeyPEM       string    `gorm:"type:text;not null"`
	Expires            *time.Time
	ProviderParameters json.RawMessage `gorm:"type:jsonb"`
}

// TableName returns the table name for ImportParams
func (ImportParams) TableName() string {
	return "import_params"
}

func (ImportParams) IsSharedModel() bool {
	return false
}

// IsExpired checks if the ImportParams has expired based on the Expires field.
func (b ImportParams) IsExpired() bool {
	if b.Expires == nil {
		return false
	}

	return time.Now().After(*b.Expires)
}
