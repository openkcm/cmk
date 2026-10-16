package model

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
)

// Keystore is an internal entity of pool item that should be persisted.
type Keystore struct {
	AutoTimeModel

	ID       uuid.UUID       `gorm:"type:uuid;primaryKey"`
	Provider string          `gorm:"type:varchar(50);not null"`
	Config   json.RawMessage `gorm:"type:jsonb;not null;unique"`
}

// TableResourceType return the authz resource type
func (m Keystore) TableResourceType() authz.RepoResourceType {
	return authz.RepoResourceTypeKeystore
}

func (m Keystore) TableName() string {
	return string(m.TableResourceType())
}

func (Keystore) IsSharedModel() bool {
	return true
}

func (m Keystore) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceType, authz.RepoAction],
	action authz.RepoAction,
) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}
