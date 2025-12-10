package tags

import (
	"github.com/google/uuid"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/utils/ptr"
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
