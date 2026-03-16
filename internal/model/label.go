package model

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
)

const ResourceID = "resource_id"

type BaseLabel struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	Key        string    `gorm:"type:varchar(255);not null"`
	Value      string    `gorm:"type:varchar(255)"`
	ResourceID uuid.UUID `gorm:"type:uuid;not null"`
}

type KeyLabel struct {
	BaseLabel
	AutoTimeModel

	CryptoKey Key `gorm:"foreignKey:ResourceID"`
}

// TableResourceType return the authz resource type
func (m KeyLabel) TableResourceType() authz.RepoResourceTypeName {
	return authz.RepoResourceTypeKeyLabel
}

func (m KeyLabel) TableName() string {
	return string(m.TableResourceType())
}

func (KeyLabel) IsSharedModel() bool {
	return false
}

func (m KeyLabel) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceTypeName, authz.RepoAction],
	action authz.RepoAction) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}
