package cmk

import (
	"context"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	tags "github.com/openkcm/cmk-core/internal/api/transform/tags"
	"github.com/openkcm/cmk-core/internal/apierrors"
	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/utils/ptr"
)

// GetTagsForKeyConfiguration returns the tags for a key configuration
func (c *APIController) GetTagsForKeyConfiguration(
	ctx context.Context,
	request cmkapi.GetTagsForKeyConfigurationRequestObject,
) (cmkapi.GetTagsForKeyConfigurationResponseObject, error) {
	t, err := c.Manager.KeyConfigTags.GetTagByKeyConfiguration(ctx, request.KeyConfigurationID)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrGettingTagsByKeyConfigurationID, err)
	}

	count := len(t)
	response := cmkapi.TagList{
		Value: make([]string, count),
	}

	if ptr.GetSafeDeref(request.Params.Count) {
		response.Count = ptr.PointTo(count)
	}

	for i, tag := range t {
		response.Value[i] = tag.Value
	}

	return cmkapi.GetTagsForKeyConfiguration200JSONResponse(response), nil
}

// AddTagsToKeyConfiguration adds tags to a key configuration
func (c *APIController) AddTagsToKeyConfiguration(
	ctx context.Context,
	request cmkapi.AddTagsToKeyConfigurationRequestObject,
) (cmkapi.AddTagsToKeyConfigurationResponseObject, error) {
	t := tags.FromAPI[*model.KeyConfigurationTag](ptr.Initializer, *request.Body)

	err := c.Manager.KeyConfigTags.CreateTagsByKeyConfiguration(ctx, request.KeyConfigurationID, t)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrCreatingTags, err)
	}

	return cmkapi.AddTagsToKeyConfiguration204Response(struct{}{}), nil
}
