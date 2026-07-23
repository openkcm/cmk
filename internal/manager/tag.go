package manager

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/model"
)

// Tags interface for managing tags on resources (currently KeyConfigurations)
type Tags interface {
	SetTags(ctx context.Context, itemID uuid.UUID, values []string) error
	GetTags(ctx context.Context, itemID uuid.UUID) ([]string, error)
	DeleteTags(ctx context.Context, itemID uuid.UUID) error
}

// TagManager is an adapter that delegates to ResourceLabelManager
// Maintains backward compatibility while using the new unified resource_labels table
type TagManager struct {
	resourceLabels ResourceLabels
}

// NewTagManager creates a new TagManager that uses ResourceLabelManager
func NewTagManager(resourceLabels ResourceLabels) *TagManager {
	return &TagManager{
		resourceLabels: resourceLabels,
	}
}

// SetTags sets tags for a key configuration
// Tags are stored as labels with key="system.tag"
func (m *TagManager) SetTags(ctx context.Context, itemID uuid.UUID, values []string) error {
	return m.resourceLabels.SetTags(ctx, model.ResourceTypeKeyConfig, itemID, values)
}

// GetTags retrieves tags for a key configuration
func (m *TagManager) GetTags(ctx context.Context, itemID uuid.UUID) ([]string, error) {
	return m.resourceLabels.GetTags(ctx, model.ResourceTypeKeyConfig, itemID)
}

// DeleteTags removes all tags for a key configuration
func (m *TagManager) DeleteTags(ctx context.Context, itemID uuid.UUID) error {
	return m.resourceLabels.DeleteTags(ctx, model.ResourceTypeKeyConfig, itemID)
}
