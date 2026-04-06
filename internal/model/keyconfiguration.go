package model

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	"github.com/openkcm/cmk/utils/identity"
)

// KeyConfiguration represents a key configuration in the database.
//
//nolint:recvcheck
type KeyConfiguration struct {
	AutoTimeModel

	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name         string    `gorm:"type:varchar(255);not null;unique"`
	Description  string    `gorm:"type:text"`
	AdminGroupID uuid.UUID `gorm:"type:uuid;not null"`
	AdminGroup   Group     `gorm:"foreignKey:AdminGroupID"`
	CreatorID    string    `gorm:"type:varchar(255);not null"`
	PrimaryKeyID *uuid.UUID
	TotalKeys    int `gorm:"->;-:migration"`
	TotalSystems int `gorm:"->;-:migration"`

	creatorName string `gorm:"-:all"`
}

// TableResourceType return the authz resource type
func (m KeyConfiguration) TableResourceType() authz.RepoResourceTypeName {
	return authz.RepoResourceTypeKeyconfiguration
}

// TableName returns the table name for KeyConfiguration
func (m KeyConfiguration) TableName() string {
	return string(m.TableResourceType())
}

func (KeyConfiguration) IsSharedModel() bool {
	return false
}

func (m KeyConfiguration) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceTypeName, authz.RepoAction],
	action authz.RepoAction,
) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}

func (kc *KeyConfiguration) SetID(id uuid.UUID) {
	kc.ID = id
}

func (kc *KeyConfiguration) GetCreatorName(
	ctx context.Context,
	identityManager identitymanagement.IdentityManagement,
) (string, error) {
	if kc.creatorName != "" {
		return kc.creatorName, nil
	}

	return identity.GetUserName(ctx, identityManager, kc.CreatorID)
}
