package label

import (
	"errors"

	"github.com/google/uuid"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/apierrors"
	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/utils/ptr"
	"github.tools.sap/kms/cmk/utils/sanitise"
)

var (
	ErrInvalidLabelDBModel      = errors.New("invalid label model")
	ErrInvalidLabelDBModelField = errors.New("invalid label model field")
)

func FromAPI(keyUUID cmkapi.KeyIDPath, apiKeyLabel cmkapi.Label) (*model.KeyLabel, error) {
	if apiKeyLabel.Key == "" {
		return nil, errs.Wrapf(apierrors.ErrNameFieldMissingProperty, "label name")
	}

	var labelValue *string
	if apiKeyLabel.Value == nil {
		labelValue = ptr.PointTo("")
	} else {
		labelValue = apiKeyLabel.Value
	}

	return &model.KeyLabel{
		BaseLabel: model.BaseLabel{
			ID:         uuid.New(),
			ResourceID: keyUUID,
			Key:        apiKeyLabel.Key,
			Value:      *labelValue,
		},
	}, nil
}

func ToAPI(modelKeyLabel *model.KeyLabel) (cmkapi.Label, error) {
	err := sanitise.Stringlikes(modelKeyLabel)
	if err != nil {
		return cmkapi.Label{}, err
	}

	if modelKeyLabel == nil {
		return cmkapi.Label{}, ErrInvalidLabelDBModel
	}

	if modelKeyLabel.Key == "" {
		return cmkapi.Label{}, errs.Wrapf(ErrInvalidLabelDBModelField, "label name")
	}

	return cmkapi.Label{
		Key:   modelKeyLabel.Key,
		Value: ptr.PointTo(modelKeyLabel.Value),
	}, nil
}
