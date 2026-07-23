package model

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/authz"
)

// ResourceType represents the type of resource that labels are attached to
type ResourceType string

const (
	// ResourceTypeKey represents labels attached to cryptographic keys
	ResourceTypeKey ResourceType = "KEY"
	// ResourceTypeKeyConfig represents labels attached to key configurations (tags)
	ResourceTypeKeyConfig ResourceType = "KEY_CONFIG"
)

// SystemTagKey is the special key used to identify tags (as opposed to regular labels)
// Tags are stored as labels with this key to distinguish them from user-defined labels
const SystemTagKey = "system.tag"

// ResourceLabel is a unified model for both labels and tags across different resource types
// Labels are key-value pairs attached to resources
// Tags are a special case of labels using SystemTagKey as the key
type ResourceLabel struct {
	AutoTimeModel

	ID           uuid.UUID    `gorm:"type:uuid;primaryKey"`
	ResourceType ResourceType `gorm:"type:varchar(50);not null"`
	ResourceID   uuid.UUID    `gorm:"type:uuid;not null"`
	Key          string       `gorm:"type:varchar(255);not null"`
	Value        string       `gorm:"type:varchar(255);not null"`
}

// TableResourceType returns the authz resource type
func (m ResourceLabel) TableResourceType() authz.RepoResourceType {
	return authz.RepoResourceTypeResourceLabel
}

func (m ResourceLabel) TableName() string {
	return string(m.TableResourceType())
}

func (ResourceLabel) IsSharedModel() bool {
	return false
}

func (m ResourceLabel) CheckAuthz(ctx context.Context,
	authzHandler *authz.Handler[authz.RepoResourceType, authz.RepoAction],
	action authz.RepoAction) (bool, error) {
	return authz.CheckAuthz(ctx, authzHandler, m.TableResourceType(), action)
}
