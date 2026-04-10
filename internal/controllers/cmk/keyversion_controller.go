package cmk

import (
	"context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/keyversion"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/utils/ptr"
)

// GetKeyVersions returns a list of key version by L1 Key ID
func (c *APIController) GetKeyVersions(ctx context.Context,
	request cmkapi.GetKeyVersionsRequestObject,
) (cmkapi.GetKeyVersionsResponseObject, error) {
	pagination := repo.Pagination{
		Skip:  ptr.GetIntOrDefault(request.Params.Skip, constants.DefaultSkip),
		Top:   ptr.GetIntOrDefault(request.Params.Top, constants.DefaultTop),
		Count: ptr.GetSafeDeref(request.Params.Count),
	}

	// Fetch the parent key to get current state for all versions
	key, err := c.Manager.Keys.Get(ctx, request.KeyID)
	if err != nil {
		return nil, apierrors.ErrQueryKeyVersionList
	}

	keyVersions, count, err := c.Manager.KeyVersions.GetKeyVersions(
		ctx,
		request.KeyID,
		pagination,
	)
	if err != nil {
		return nil, apierrors.ErrQueryKeyVersionList
	}

	// Get the latest version from first element (already sorted by rotated_at DESC)
	var latestVersionID string
	if len(keyVersions) > 0 {
		latestVersionID = keyVersions[0].ID.String()
	}

	// Convert each Key Version to its response format
	response := make([]cmkapi.KeyVersion, 0, len(keyVersions))
	keyState := cmkapi.KeyState(key.State)

	for _, kv := range keyVersions {
		apiKv, err := keyversion.ToAPI(*kv)
		if err != nil {
			return nil, apierrors.ErrTransformKeyVersionList
		}

		// Set isPrimary by comparing with the latest version ID
		isPrimary := kv.ID.String() == latestVersionID
		apiKv.IsPrimary = &isPrimary

		// Set state from parent key (all versions share the same state)
		apiKv.State = &keyState

		response = append(response, *apiKv)
	}

	apiresponse := cmkapi.KeyVersionList{Value: response}

	if ptr.GetSafeDeref(request.Params.Count) {
		apiresponse.Count = ptr.PointTo(count)
	}

	return cmkapi.GetKeyVersions200JSONResponse(apiresponse), nil
}
