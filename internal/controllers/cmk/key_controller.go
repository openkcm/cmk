package cmk

import (
	"context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/importparams"
	keyTransform "github.com/openkcm/cmk/internal/api/transform/key"
	"github.com/openkcm/cmk/internal/api/transform/key/keyshared"
	"github.com/openkcm/cmk/internal/api/transform/key/transformer"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/utils/ptr"
)

// PostKeys handles the creation of a new key
func (c *APIController) PostKeys(ctx context.Context,
	request cmkapi.PostKeysRequestObject,
) (cmkapi.PostKeysResponseObject, error) {
	if request.Body.Provider == nil {
		if request.Body.Type == cmkapi.KeyTypeHYOK {
			return nil, errs.Wrap(apierrors.ErrTransformKeyFromAPI, keyshared.ErrProviderIsRequired)
		}

		defaultProvider, err := c.Manager.Keys.GetDefaultKeystoreFromCatalog()
		if err != nil {
			return nil, errs.Wrap(apierrors.ErrDefaultKeystoreNotFound, err)
		}

		request.Body.Provider = &defaultProvider
	}

	providerTransformer, err := transformer.NewPluginProviderTransformer(c.pluginCatalog, *request.Body.Provider)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformKeyFromAPI, err)
	}

	dbKey, err := keyTransform.FromAPI(ctx, *request.Body, *providerTransformer)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformKeyFromAPI, err)
	}

	dbKey, err = c.Manager.Keys.Create(ctx, dbKey)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrCreateKey, err)
	}

	// Get the created key from the database
	dbKey, err = c.Manager.Keys.Get(ctx, dbKey.ID)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrCreateKey, err)
	}

	cmkAPIKey, err := keyTransform.ToAPI(*dbKey)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformKeyToAPI, err)
	}

	return cmkapi.PostKeys201JSONResponse(*cmkAPIKey), nil
}

// GetKeys handles retrieving all keys
func (c *APIController) GetKeys(ctx context.Context,
	request cmkapi.GetKeysRequestObject,
) (cmkapi.GetKeysResponseObject, error) {
	keyConfigID := request.Params.KeyConfigurationID
	skip := ptr.GetIntOrDefault(request.Params.Skip, constants.DefaultSkip)
	top := ptr.GetIntOrDefault(request.Params.Top, constants.DefaultTop)

	keys, total, err := c.Manager.Keys.GetKeys(ctx, ptr.PointTo(keyConfigID), skip, top)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrQueryKeyList, err)
	}

	// Convert each Key to its response format
	values := make([]cmkapi.Key, len(keys))

	for i, dbKey := range keys {
		cmkAPIKey, err := keyTransform.ToAPI(*dbKey)
		if err != nil {
			return nil, errs.Wrap(apierrors.ErrTransformKeyToAPI, err)
		}

		// cmkAPIKey is never nil at this point
		values[i] = *cmkAPIKey
	}

	response := cmkapi.KeyList{
		Value: values,
	}

	if ptr.GetSafeDeref(request.Params.Count) {
		response.Count = ptr.PointTo(total)
	}

	return cmkapi.GetKeys200JSONResponse(response), err
}

// DeleteKeysKeyID handles deleting a key by its ID
func (c *APIController) DeleteKeysKeyID(ctx context.Context,
	request cmkapi.DeleteKeysKeyIDRequestObject,
) (cmkapi.DeleteKeysKeyIDResponseObject, error) {
	if c.Manager.Workflow.IsWorkflowEnabled(ctx) {
		return nil, apierrors.ErrActionRequireWorkflow
	}

	err := c.Manager.Keys.Delete(ctx, request.KeyID)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrDeleteKey, err)
	}

	return cmkapi.DeleteKeysKeyID204Response(struct{}{}), nil
}

// GetKeysKeyID handles retrieving a key by its ID
func (c *APIController) GetKeysKeyID(ctx context.Context,
	request cmkapi.GetKeysKeyIDRequestObject,
) (cmkapi.GetKeysKeyIDResponseObject, error) {
	dbKey, err := c.Manager.Keys.Get(ctx, request.KeyID)
	if err != nil {
		return nil, err
	}

	cmkAPIKey, err := keyTransform.ToAPI(*dbKey)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformKeyToAPI, err)
	}

	return cmkapi.GetKeysKeyID200JSONResponse(*cmkAPIKey), nil
}

// UpdateKey handles updating an existing key
func (c *APIController) UpdateKey(ctx context.Context,
	request cmkapi.UpdateKeyRequestObject,
) (cmkapi.UpdateKeyResponseObject, error) {
	if request.Body.Enabled != nil && c.Manager.Workflow.IsWorkflowEnabled(ctx) {
		return nil, apierrors.ErrActionRequireWorkflow
	}

	dbKey, err := c.Manager.Keys.UpdateKey(ctx, request.KeyID, *request.Body)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrUpdateKey, err)
	}

	cmkAPIKey, err := keyTransform.ToAPI(*dbKey)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformKeyToAPI, err)
	}

	return cmkapi.UpdateKey200JSONResponse(*cmkAPIKey), nil
}

func (c *APIController) GetKeyImportParams(ctx context.Context,
	request cmkapi.GetKeyImportParamsRequestObject,
) (cmkapi.GetKeyImportParamsResponseObject, error) {
	importParams, err := c.Manager.Keys.GetImportParams(
		ctx, request.KeyID)
	if err != nil {
		return nil, err
	}

	importParamsAPI, err := importparams.ToAPI(*importParams)
	if err != nil {
		return nil, err
	}

	return cmkapi.GetKeyImportParams200JSONResponse(*importParamsAPI), nil
}

func (c *APIController) ImportKeyMaterial(ctx context.Context,
	request cmkapi.ImportKeyMaterialRequestObject,
) (cmkapi.ImportKeyMaterialResponseObject, error) {
	dbKey, err := c.Manager.Keys.ImportKeyMaterial(ctx, request.KeyID, request.Body.WrappedKeyMaterial)
	if err != nil {
		return nil, err
	}

	cmkAPIKey, err := keyTransform.ToAPI(*dbKey)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformKeyToAPI, err)
	}

	return cmkapi.ImportKeyMaterial201JSONResponse(*cmkAPIKey), nil
}
