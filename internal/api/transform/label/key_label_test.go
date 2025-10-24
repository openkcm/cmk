package label_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/api/transform"
	"github.com/openkcm/cmk-core/internal/api/transform/label"
	"github.com/openkcm/cmk-core/internal/apierrors"
	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/utils/ptr"
)

type labelsTransformerTestCase struct {
	name        string
	expectedErr error
}

func TestTransformLabel_FromAPI(t *testing.T) {
	type labelsTransformerFromAPITestCase struct {
		labelsTransformerTestCase

		inputKeyID         uuid.UUID
		inputLabelName     string
		inputLabelValuePtr *string
		expectedLabelValue string
	}

	tcs := []labelsTransformerFromAPITestCase{
		{
			labelsTransformerTestCase: labelsTransformerTestCase{
				name:        "Valid_KeyID_And_Labels_From_API_Request",
				expectedErr: nil,
			},
			inputKeyID:         uuid.New(),
			inputLabelName:     "foo",
			inputLabelValuePtr: ptr.PointTo("bar"),
			expectedLabelValue: "bar",
		},
		{
			labelsTransformerTestCase: labelsTransformerTestCase{
				name:        "Label_Value_As_Empty_String",
				expectedErr: nil,
			},
			inputKeyID:         uuid.New(),
			inputLabelName:     "foo",
			inputLabelValuePtr: ptr.PointTo(""),
			expectedLabelValue: "",
		},
		{
			labelsTransformerTestCase: labelsTransformerTestCase{
				name:        "Label_Value_As_nil",
				expectedErr: nil,
			},
			inputKeyID:         uuid.New(),
			inputLabelName:     "foo",
			inputLabelValuePtr: nil,
			expectedLabelValue: "",
		},
		{
			labelsTransformerTestCase: labelsTransformerTestCase{
				name:        "Label_Name_As_Empty_String",
				expectedErr: errs.Wrapf(apierrors.ErrNameFieldMissingProperty, "label name"),
			},
			inputKeyID:         uuid.New(),
			inputLabelName:     "",
			inputLabelValuePtr: ptr.PointTo("bar"),
		},
		{
			labelsTransformerTestCase: labelsTransformerTestCase{
				name: "Label_Name_Longer_Than_Specified_Length",
				expectedErr: errs.Wrapf(
					transform.ErrAPIInvalidProperty,
					fmt.Sprintf(
						"label name must be less than or equal to %d characters",
						label.MaxLengthOfLabelName,
					),
				),
			},
			inputKeyID:         uuid.New(),
			inputLabelName:     strings.Repeat("a", label.MaxLengthOfLabelName+1),
			inputLabelValuePtr: ptr.PointTo("bar"),
		},
		{
			labelsTransformerTestCase: labelsTransformerTestCase{
				name: "Label_Value_Longer_Than_Specified_Length",
				expectedErr: errs.Wrapf(
					transform.ErrAPIInvalidProperty,
					fmt.Sprintf(
						"label value must be less than or equal to %d characters",
						label.MaxLengthOfLabelValue,
					),
				),
			},
			inputKeyID:         uuid.New(),
			inputLabelName:     "foo",
			inputLabelValuePtr: ptr.PointTo(strings.Repeat("a", label.MaxLengthOfLabelValue+1)),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// PREPARE
			labelAPI := cmkapi.Label{
				Key:   tc.inputLabelName,
				Value: tc.inputLabelValuePtr,
			}

			// EXECUTE
			gotLabel, gotErr := label.FromAPI(tc.inputKeyID, labelAPI)

			// VERIFY
			if tc.expectedErr != nil {
				assert.Error(t, gotErr)
				assert.EqualError(t, gotErr, tc.expectedErr.Error())
				assert.Nil(t, gotLabel)
			} else {
				assert.Equal(t, tc.inputKeyID, gotLabel.ResourceID)
				assert.NotEqual(t, uuid.Nil, gotLabel.ID)
				assert.NotEqual(t, tc.inputKeyID, gotLabel.ID)
				assert.LessOrEqual(t, len(gotLabel.Key), label.MaxLengthOfLabelName)
				assert.LessOrEqual(t, len(gotLabel.Value), label.MaxLengthOfLabelValue)
				assert.Equal(t, tc.inputLabelName, gotLabel.Key)
				assert.Equal(t, tc.expectedLabelValue, gotLabel.Value)
			}
		})
	}
}

func TestTransformLabel_ToAPI(t *testing.T) {
	// PREPARE
	labelGen := func(labelName, labelValue string) *model.KeyLabel {
		return &model.KeyLabel{
			BaseLabel: model.BaseLabel{
				ID:         uuid.New(),
				Key:        labelName,
				Value:      labelValue,
				ResourceID: uuid.New(),
			},
			CryptoKey: model.Key{},
		}
	}

	type labelTransformerToAPITestCase struct {
		labelsTransformerTestCase

		inputLabelModel    *model.KeyLabel
		expectedLabelName  string
		expectedLabelValue string
	}

	tcs := []labelTransformerToAPITestCase{
		{
			labelsTransformerTestCase: labelsTransformerTestCase{
				name:        "Valid_DB_Model_Label_Name_And_Value",
				expectedErr: nil,
			},
			inputLabelModel:    labelGen("foo", "bar"),
			expectedLabelName:  "foo",
			expectedLabelValue: "bar",
		},
		{
			labelsTransformerTestCase: labelsTransformerTestCase{
				name:        "DB_Model_Label_With_Empty_Label_Value",
				expectedErr: nil,
			},
			inputLabelModel:    labelGen("foo", ""),
			expectedLabelName:  "foo",
			expectedLabelValue: "",
		},
		{
			labelsTransformerTestCase: labelsTransformerTestCase{
				name:        "Nil_DB_Model_Label",
				expectedErr: label.ErrInvalidLabelDBModel,
			},
			inputLabelModel: nil,
		},
		{
			labelsTransformerTestCase: labelsTransformerTestCase{
				name:        "DB_Model_Label_With_Empty_Label_Name",
				expectedErr: errs.Wrapf(label.ErrInvalidLabelDBModelField, "label name"),
			},
			inputLabelModel: labelGen("", "bar"),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// EXECUTE
			gotLabel, gotErr := label.ToAPI(tc.inputLabelModel)

			// VERIFY
			if tc.expectedErr != nil {
				assert.Error(t, gotErr)
				assert.EqualError(t, gotErr, tc.expectedErr.Error())
			} else {
				assert.Equal(t, tc.expectedLabelName, gotLabel.Key)
				assert.NotNil(t, gotLabel.Value)
				assert.Equal(t, tc.expectedLabelValue, *gotLabel.Value)
			}
		})
	}
}
