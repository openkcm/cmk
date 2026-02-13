package cmk

import (
	"context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/clientcertificates"
	"github.com/openkcm/cmk/internal/api/transform/keyconfiguration"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

// GetKeyConfigurations returns the key configurations
func (c *APIController) GetKeyConfigurations(
	ctx context.Context,
	r cmkapi.GetKeyConfigurationsRequestObject,
) (cmkapi.GetKeyConfigurationsResponseObject, error) {
	pagination := repo.Pagination{
		Skip:  ptr.GetIntOrDefault(r.Params.Skip, constants.DefaultSkip),
		Top:   ptr.GetIntOrDefault(r.Params.Top, constants.DefaultTop),
		Count: ptr.GetSafeDeref(r.Params.Count),
	}

	expand := r.Params.ExpandGroup != nil && *r.Params.ExpandGroup

	filter := manager.KeyConfigFilter{Pagination: pagination, Expand: expand}

	keyConfigs, total, err := c.Manager.KeyConfig.GetKeyConfigurations(ctx, filter)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrGettingKeyConfig, err)
	}

	values := make([]cmkapi.KeyConfiguration, len(keyConfigs))

	for i, dbConfig := range keyConfigs {
		apiConfig, err := keyconfiguration.ToAPI(*dbConfig)
		if err != nil {
			return nil, errs.Wrap(apierrors.ErrTransformKeyConfigurationList, err)
		}

		values[i] = *apiConfig
	}

	response := cmkapi.KeyConfigurationList{
		Value: values,
	}

	if pagination.Count {
		response.Count = ptr.PointTo(total)
	}

	return cmkapi.GetKeyConfigurations200JSONResponse(response), nil
}

// PostKeyConfigurations creates a new key configuration
func (c *APIController) PostKeyConfigurations(
	ctx context.Context,
	request cmkapi.PostKeyConfigurationsRequestObject,
) (cmkapi.PostKeyConfigurationsResponseObject, error) {
	keyConfig, err := keyconfiguration.FromAPI(*request.Body)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformKeyConfigurationFromAPI, err)
	}

	clientData, err := cmkcontext.ExtractClientData(ctx)
	if err != nil {
		return nil, errs.Wrap(err, apierrors.ErrClientDataInvalid)
	}

	keyConfig.CreatorID = clientData.Identifier
	keyConfig.CreatorName = clientData.Email

	keyConfig, err = c.Manager.KeyConfig.PostKeyConfigurations(ctx, keyConfig)
	if err != nil {
		return nil, err
	}

	response, err := keyconfiguration.ToAPI(*keyConfig)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformKeyConfigurationToAPI, err)
	}

	return cmkapi.PostKeyConfigurations201JSONResponse(*response), nil
}

// DeleteKeyConfigurationByID deletes a key configuration by ID
func (c *APIController) DeleteKeyConfigurationByID(
	ctx context.Context,
	request cmkapi.DeleteKeyConfigurationByIDRequestObject,
) (cmkapi.DeleteKeyConfigurationByIDResponseObject, error) {
	err := c.Manager.KeyConfig.DeleteKeyConfigurationByID(ctx, request.KeyConfigurationID)
	if err != nil {
		return nil, err
	}

	return cmkapi.DeleteKeyConfigurationByID204Response(struct{}{}), nil
}

// GetKeyConfigurationByID returns a key configuration by ID
func (c *APIController) GetKeyConfigurationByID(
	ctx context.Context,
	request cmkapi.GetKeyConfigurationByIDRequestObject,
) (cmkapi.GetKeyConfigurationByIDResponseObject, error) {
	keyConfig, err := c.Manager.KeyConfig.GetKeyConfigurationByID(ctx, request.KeyConfigurationID)
	if err != nil {
		return nil, err
	}

	response, err := keyconfiguration.ToAPI(*keyConfig)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformKeyConfigurationToAPI, err)
	}

	return cmkapi.GetKeyConfigurationByID200JSONResponse(*response), nil
}

// UpdateKeyConfigurationByID updates a key configuration by ID
func (c *APIController) UpdateKeyConfigurationByID(
	ctx context.Context,
	request cmkapi.UpdateKeyConfigurationByIDRequestObject,
) (cmkapi.UpdateKeyConfigurationByIDResponseObject, error) {
	keyConfig, err := c.Manager.KeyConfig.UpdateKeyConfigurationByID(ctx, request.KeyConfigurationID, *request.Body)
	if err != nil {
		return nil, err
	}

	response, err := keyconfiguration.ToAPI(*keyConfig)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformKeyConfigurationToAPI, err)
	}

	return cmkapi.UpdateKeyConfigurationByID200JSONResponse(*response), nil
}

func (c *APIController) GetKeyConfigurationsCertificates(
	ctx context.Context,
	_ cmkapi.GetKeyConfigurationsCertificatesRequestObject,
) (cmkapi.GetKeyConfigurationsCertificatesResponseObject, error) {
	clientCerts, err := c.Manager.KeyConfig.GetClientCertificates(ctx)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrGetClientCertificates, err)
	}

	response, err := clientcertificates.ToAPI(clientCerts)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrGetClientCertificates, err)
	}

	return cmkapi.GetKeyConfigurationsCertificates200JSONResponse(*response), nil
}
