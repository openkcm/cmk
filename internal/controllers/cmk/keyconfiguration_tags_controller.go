package cmk

import (
	"context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/utils/ptr"
	"github.com/openkcm/cmk/utils/sanitise"
)

// GetTagsForKeyConfiguration returns the tags for a key configuration
func (c *APIController) GetTagsForKeyConfiguration(
	ctx context.Context,
	request cmkapi.GetTagsForKeyConfigurationRequestObject,
) (cmkapi.GetTagsForKeyConfigurationResponseObject, error) {
	tags, err := c.Manager.Tags.GetTags(ctx, request.KeyConfigurationID)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrGettingTagsByKeyConfigurationID, err)
	}

	err = sanitise.Stringlikes(&tags)
	if err != nil {
		return nil, err
	}

	count := len(tags)
	response := cmkapi.TagList{
		Value: tags,
	}

	if ptr.GetSafeDeref(request.Params.Count) {
		response.Count = ptr.PointTo(count)
	}

	return cmkapi.GetTagsForKeyConfiguration200JSONResponse(response), nil
}

// AddTagsToKeyConfiguration adds tags to a key configuration
func (c *APIController) AddTagsToKeyConfiguration(
	ctx context.Context,
	request cmkapi.AddTagsToKeyConfigurationRequestObject,
) (cmkapi.AddTagsToKeyConfigurationResponseObject, error) {
	err := c.Manager.Tags.SetTags(ctx, request.KeyConfigurationID, request.Body.Tags)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrCreatingTags, err)
	}

	return cmkapi.AddTagsToKeyConfiguration204Response(struct{}{}), nil
}
