package model

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
)

// ResourceType defines the type of resource that can have labels
type ResourceType string

const (
	// ResourceTypeKeyConfig represents a key configuration resource
	ResourceTypeKeyConfig ResourceType = "key_configuration"
)

// SystemTagKey is the reserved key for system tags
const SystemTagKey = "system.tag"

// ResourceLabel represents a generic label on any resource
// Used for double-write during migration from separate labels/tags tables
type ResourceLabel struct {
	ID           uuid.UUID    `gorm:"type:uuid;primaryKey"`
	ResourceType ResourceType `gorm:"type:text;not null;index:idx_resource_labels_resource"`
	ResourceID   uuid.UUID    `gorm:"type:uuid;not null;index:idx_resource_labels_resource"`
	Key          string       `gorm:"type:text;not null;index:idx_resource_labels_key"`
	Value        string       `gorm:"type:text;not null"`
	CreatedAt    time.Time    `gorm:"not null;default:now()"`
	UpdatedAt    time.Time    `gorm:"not null;default:now()"`
}

// TableName returns the table name for ResourceLabel
func (ResourceLabel) TableName() string {
	return "resource_labels"
}

// IsSharedModel returns false since ResourceLabel is tenant-scoped
func (ResourceLabel) IsSharedModel() bool {
	return false
}

// TableResourceType returns the authz resource type
func (ResourceLabel) TableResourceType() authz.RepoResourceType {
	return authz.RepoResourceTypeResourceLabel
}

// CheckAuthz checks authorization for the resource label
func (m ResourceLabel) CheckAuthz(
	ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceType, authz.RepoAction],
	action authz.RepoAction,
) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}
