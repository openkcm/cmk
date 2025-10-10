package cmk

import (
	"context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform"
	"github.com/openkcm/cmk/internal/api/transform/keyversion"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/utils/ptr"
)

// GetKeyVersions returns a list of key version by L1 Key ID
func (c *APIController) GetKeyVersions(ctx context.Context,
	request cmkapi.GetKeyVersionsRequestObject,
) (cmkapi.GetKeyVersionsResponseObject, error) {
	skip := ptr.GetIntOrDefault(request.Params.Skip, constants.DefaultSkip)
	top := ptr.GetIntOrDefault(request.Params.Top, constants.DefaultTop)

	keyVersions, count, err := c.Manager.KeyVersions.GetKeyVersions(
		ctx,
		request.KeyID,
		skip,
		top,
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

// CreateKeyVersion creates a new key version for L1 Key ID
func (c *APIController) CreateKeyVersion(ctx context.Context,
	request cmkapi.CreateKeyVersionRequestObject,
) (cmkapi.CreateKeyVersionResponseObject, error) {
	keyVersion, err := c.Manager.KeyVersions.CreateKeyVersion(ctx, request.KeyID, request.Body.NativeID)
	if err != nil || keyVersion == nil {
		return nil, errs.Wrap(apierrors.ErrCreateKeyVersion, err)
	}

	response, err := keyversion.ToAPI(*keyVersion)
	if err != nil {
		return nil, apierrors.ErrTransformKeyVersionToAPI
	}

	return cmkapi.CreateKeyVersion201JSONResponse(*response), nil
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
