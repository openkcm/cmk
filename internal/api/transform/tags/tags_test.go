package tags_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/tags"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
)

func TestFromAPI(t *testing.T) {
	apiKey := cmkapi.Tags{
		Tags: []string{"tag1", "tag2"},
	}

	result := tags.FromAPI[*model.KeyConfigurationTag](ptr.Initializer, apiKey)

	assert.Len(t, result, 2)
	assert.Equal(t, "tag1", result[0].Value)
	assert.Equal(t, "tag2", result[1].Value)
	assert.NotEqual(t, uuid.Nil, result[0].ID)
	assert.NotEqual(t, uuid.Nil, result[1].ID)
}
