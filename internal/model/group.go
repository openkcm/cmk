package model

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/constants"
)

type Group struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey"`
	Name          string         `gorm:"type:varchar(255);not null;unique"`
	Description   string         `gorm:"type:text"`
	Role          constants.Role `gorm:"type:varchar(255);not null"`
	IAMIdentifier string         `gorm:"type:varchar;not null;unique"`
}

func NewIAMIdentifier(name string, tenantID string) string {
	return fmt.Sprintf("%s_%s_%s", constants.KMS, name, tenantID)
}

// TableName returns the table name for Key
func (Group) TableName() string {
	return "group"
}

func (Group) IsSharedModel() bool {
	return false
}
