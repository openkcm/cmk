package manager

import (
	"context"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

type KeyConfigurationTag interface {
	GetTagByKeyConfiguration(ctx context.Context, keyConfigurationID uuid.UUID) ([]model.KeyConfigurationTag, error)
	CreateTagsByKeyConfiguration(ctx context.Context, keyConfigurationID uuid.UUID,
		tags []*model.KeyConfigurationTag) error
}

type KeyConfigurationTagManager struct {
	repo repo.Repo
}

func NewKeyConfigurationTagManager(
	repository repo.Repo,
) *KeyConfigurationTagManager {
	return &KeyConfigurationTagManager{
		repo: repository,
	}
}

// GetTagByKeyConfiguration returns tags for a key configuration.
func (m *KeyConfigurationTagManager) GetTagByKeyConfiguration(
	ctx context.Context,
	keyConfigurationID uuid.UUID,
) ([]model.KeyConfigurationTag, error) {
	keyConfig := model.KeyConfiguration{}

	err := getTags[*model.KeyConfiguration](ctx, m.repo, keyConfigurationID, &keyConfig)
	if err != nil {
		return nil, err
	}

	return keyConfig.Tags, nil
}

// CreateTagsByKeyConfiguration creates tags for a key configuration. If tags already exist, they are replaced.
func (m *KeyConfigurationTagManager) CreateTagsByKeyConfiguration(
	ctx context.Context,
	keyConfigurationID uuid.UUID,
	tags []*model.KeyConfigurationTag,
) error {
	return createTags[*model.KeyConfiguration](ctx, m.repo, &model.KeyConfiguration{},
		keyConfigurationID, tags)
}
