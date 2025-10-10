package label

import (
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/ptr"
)

const (
	MaxLengthOfLabelName  = 255
	MaxLengthOfLabelValue = 255
)

var (
	ErrInvalidLabelDBModel      = errors.New("invalid label model")
	ErrInvalidLabelDBModelField = errors.New("invalid label model field")
)

func FromAPI(keyUUID cmkapi.KeyIDPath, apiKeyLabel cmkapi.Label) (*model.KeyLabel, error) {
	if apiKeyLabel.Key == "" {
		return nil, errs.Wrapf(apierrors.ErrNameFieldMissingProperty, "label name")
	}

	if len(apiKeyLabel.Key) > MaxLengthOfLabelName {
		return nil, errs.Wrapf(
			transform.ErrAPIInvalidProperty,
			fmt.Sprintf(
				"label name must be less than or equal to %d characters",
				MaxLengthOfLabelName,
			),
		)
	}

	var labelValue *string
	if apiKeyLabel.Value == nil {
		labelValue = ptr.PointTo("")
	} else {
		labelValue = apiKeyLabel.Value
	}

	if len(*labelValue) > MaxLengthOfLabelValue {
		return nil, errs.Wrapf(
			transform.ErrAPIInvalidProperty,
			fmt.Sprintf(
				"label value must be less than or equal to %d characters",
				MaxLengthOfLabelValue,
			),
		)
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
