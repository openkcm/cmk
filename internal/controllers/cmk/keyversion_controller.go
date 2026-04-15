package cmk

import (
	"context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform"
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
		Skip:  ptr.GetPtrOrDefault(request.Params.Skip, constants.DefaultSkip),
		Top:   ptr.GetPtrOrDefault(request.Params.Top, constants.DefaultTop),
		Count: ptr.GetSafeDeref(request.Params.Count),
	}

	keyVersions, count, err := c.Manager.KeyVersions.GetKeyVersions(
		ctx,
		request.KeyID,
		pagination,
	)
	if err != nil {
		return nil, apierrors.ErrQueryKeyVersionList
	}

	// Convert each Key Version to its response format
	response, err := transform.ToList(
		keyVersions,
		keyversion.ToAPI,
	)
	if err != nil {
		return nil, apierrors.ErrTransformKeyVersionList
	}

	apiresponse := cmkapi.KeyVersionList{Value: response}

	if ptr.GetSafeDeref(request.Params.Count) {
		apiresponse.Count = ptr.PointTo(count)
	}

	return cmkapi.GetKeyVersions200JSONResponse(apiresponse), nil
}

// GetKeyVersionByNumber returns a key version by key version number and L1 key ID
func (c *APIController) GetKeyVersionByNumber(
	ctx context.Context,
	request cmkapi.GetKeyVersionByNumberRequestObject,
) (cmkapi.GetKeyVersionByNumberResponseObject, error) {
	keyVersion, err := c.Manager.KeyVersions.GetByKeyIDAndByNumber(
		ctx,
		request.KeyID,
		request.Version,
	)
	if err != nil {
		return nil, apierrors.ErrGettingKeyVersionByNumber
	}

	response, err := keyversion.ToAPI(*keyVersion)
	if err != nil {
		return nil, apierrors.ErrTransformKeyVersionToAPI
	}

	return cmkapi.GetKeyVersionByNumber200JSONResponse(*response), nil
}
