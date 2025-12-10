package model

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.tools.sap/kms/cmk/internal/constants"
)

var (
	ErrInvalidIAMIdentifier = errors.New("invalid group IAMIdentifier")
	ErrInvalidName          = errors.New("invalid group name")
)

const (
	MaxIAMIdentifierLength = 128
	MaxNameLength          = 64

	// ValidTextPattern is a pattern matching
	// alphanumeric, "_" and "-"
	ValidTextPattern = `^[a-zA-Z0-9 _-]+$`
)

//nolint:recvcheck
type Group struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey"`
	Name          string         `gorm:"type:varchar(64);not null;unique"`
	Description   string         `gorm:"type:text"`
	Role          constants.Role `gorm:"type:varchar(255);not null"`
	IAMIdentifier string         `gorm:"type:varchar(128);not null;unique"`
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

// BeforeSave is ran before any creating/updating the group
// but before finishing the transaction
// If this step fails the transaction should be aborted
func (g *Group) BeforeSave(_ *gorm.DB) error {
	textValidator := regexp.MustCompile(ValidTextPattern)

	if !textValidator.MatchString(g.IAMIdentifier) || len(g.IAMIdentifier) > MaxIAMIdentifierLength {
		return ErrInvalidIAMIdentifier
	}

	if !textValidator.MatchString(g.Name) || len(g.Name) > MaxNameLength {
		return ErrInvalidName
	}

	return nil
}
