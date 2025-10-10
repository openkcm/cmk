package model

import "encoding/json"

// TenantConfig represents a key in the database.
type TenantConfig struct {
	Key   string          `gorm:"type:varchar(255);primaryKey"`
	Value json.RawMessage `gorm:"type:jsonb;not null"`
}

// TableName returns the table name for Key
func (TenantConfig) TableName() string {
	return "tenant_configs"
}

func (TenantConfig) IsSharedModel() bool {
	return false
}
