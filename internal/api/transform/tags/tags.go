package tags

import (
	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
)

type TagSetter interface {
	SetTag(tag model.BaseTag)
}

func FromAPI[T TagSetter](p ptr.InitializerFunc[T], apiKey cmkapi.Tags) []T {
	tags := make([]T, len(apiKey.Tags))
	for i := range apiKey.Tags {
		tags[i] = p()
		tags[i].SetTag(
			model.BaseTag{
				ID:    uuid.New(),
				Value: apiKey.Tags[i],
			})
	}

	return tags
}
