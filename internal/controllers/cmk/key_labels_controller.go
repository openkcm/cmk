package cmk

import (
	"context"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	keylabel "github.com/openkcm/cmk-core/internal/api/transform/label"
	"github.com/openkcm/cmk-core/internal/apierrors"
	"github.com/openkcm/cmk-core/internal/constants"
	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/utils/ptr"
)

// GetKeyLabels handles fetching a list of all the labels attached to the key.
func (c *APIController) GetKeyLabels(ctx context.Context,
	r cmkapi.GetKeyLabelsRequestObject,
) (cmkapi.GetKeyLabelsResponseObject, error) {
	skip := ptr.GetIntOrDefault(r.Params.Skip, constants.DefaultSkip)
	top := ptr.GetIntOrDefault(r.Params.Top, constants.DefaultTop)

	labelsDBModels, total, err := c.Manager.Labels.GetKeyLabels(ctx, r.KeyID, skip, top)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrGetKeyLabels, err)
	}

	labels := make([]cmkapi.Label, 0)

	for _, labelDBModel := range labelsDBModels {
		label, err := keylabel.ToAPI(labelDBModel)
		if err != nil {
			return nil, errs.Wrap(apierrors.ErrTransformLabelList, err)
		}

		labels = append(labels, label)
	}

	response := cmkapi.LabelList{
		Value: labels,
	}

	if ptr.GetSafeDeref(r.Params.Count) {
		response.Count = total
	}

	return cmkapi.GetKeyLabels200JSONResponse(response), nil
}

// CreateOrUpdateLabels handle adding a new label to the key if does not already exist
// otherwise update them.
func (c *APIController) CreateOrUpdateLabels(
	ctx context.Context,
	request cmkapi.CreateOrUpdateLabelsRequestObject,
) (cmkapi.CreateOrUpdateLabelsResponseObject, error) {
	if request.Body != nil && len(*request.Body) == 0 {
		return nil, apierrors.ErrEmptyInputLabel
	}

	labels := make([]*model.KeyLabel, len(*request.Body))

	for i, labelReq := range *request.Body {
		label, err := keylabel.FromAPI(request.KeyID, labelReq)
		if err != nil {
			return nil, errs.Wrap(apierrors.ErrTransformLabelFromAPI, err)
		}

		labels[i] = label
	}

	err := c.Manager.Labels.CreateOrUpdateLabel(ctx, request.KeyID, labels)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrUpdateLabel, err)
	}

	return cmkapi.CreateOrUpdateLabels204Response(struct{}{}), nil
}

// DeleteLabel handles deleting the label attached to the key specified by its ID and a label is specified by its name.
func (c *APIController) DeleteLabel(ctx context.Context,
	request cmkapi.DeleteLabelRequestObject,
) (cmkapi.DeleteLabelResponseObject, error) {
	ok, err := c.Manager.Labels.DeleteLabel(ctx, request.KeyID, request.LabelName)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrDeleteLabel, err)
	}

	if !ok {
		return nil, apierrors.ErrLabelNotFound
	}

	return cmkapi.DeleteLabel204Response(struct{}{}), nil
}
