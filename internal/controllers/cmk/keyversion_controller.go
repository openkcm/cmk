package cmk

import (
	"context"
	"errors"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/keyversion"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/utils/ptr"
)

// GetKeyVersions returns a list of key version by L1 Key ID
//
//nolint:cyclop
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
		return nil, err
	}

	// Get the latest version (independent of pagination) to determine isPrimary correctly.
	// Only allow "no versions found" to continue (will return empty list),
	// but propagate all other errors (DB failures, permissions, etc.)
	latestVersion, err := c.Manager.KeyVersions.GetLatestVersion(ctx, request.KeyID)
	if err != nil && !errors.Is(err, manager.ErrNoKeyVersionsFound) {
		// Real error (not just "no versions") - propagate it
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

	// If no versions exist, return empty list (latestVersion will be nil)
	if latestVersion == nil || len(keyVersions) == 0 {
		apiresponse := cmkapi.KeyVersionList{Value: []cmkapi.KeyVersion{}}
		if ptr.GetSafeDeref(request.Params.Count) {
			apiresponse.Count = ptr.PointTo(0)
		}
		return cmkapi.GetKeyVersions200JSONResponse(apiresponse), nil
	}

	// Convert each Key Version to its response format
	response := make([]cmkapi.KeyVersion, 0, len(keyVersions))
	keyState := cmkapi.KeyState(key.State)

	for _, kv := range keyVersions {
		apiKv, err := keyversion.ToAPI(*kv, latestVersion.ID, keyState)
		if err != nil {
			return nil, apierrors.ErrTransformKeyVersionList
		}

		response = append(response, *apiKv)
	}

	apiresponse := cmkapi.KeyVersionList{Value: response}

	if ptr.GetSafeDeref(request.Params.Count) {
		apiresponse.Count = ptr.PointTo(count)
	}

	return cmkapi.GetKeyVersions200JSONResponse(apiresponse), nil
}
