package model

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
)

type Tag struct {
	ID     uuid.UUID       `gorm:"type:uuid;primaryKey"` // ID of the Item
	Values json.RawMessage `gorm:"type:jsonb"`
}

// TableResourceType return the authz resource type
func (m Tag) TableResourceType() authz.RepoResourceTypeName {
	return authz.RepoResourceTypeTag
}

func (m Tag) TableName() string {
	return string(m.TableResourceType())
}

func (Tag) IsSharedModel() bool {
	return false
}

func (m Tag) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceTypeName, authz.RepoAction],
	action authz.RepoAction) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}
