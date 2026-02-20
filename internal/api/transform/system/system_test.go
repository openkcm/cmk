package system_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/system"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/utils/ptr"
)

func TestToAPI(t *testing.T) {
	tests := []struct {
		name           string
		sys            model.System
		sysCfg         *config.System
		updateExpected func(s model.System, sApi *cmkapi.System)
	}{
		{
			name: "With system properties",
			sys: *testutils.NewSystem(func(s *model.System) {
				s.Properties = map[string]string{"test": "test"}
			}),
			sysCfg: &config.System{OptionalProperties: map[string]config.SystemProperty{"test": {DisplayName: "test-display"}}},
			updateExpected: func(s model.System, sApi *cmkapi.System) {
				sApi.Properties = &map[string]any{"test": "test"}
			},
		},
		{
			name:           "Without system properties",
			sys:            *testutils.NewSystem(func(s *model.System) {}),
			updateExpected: func(s model.System, sApi *cmkapi.System) {},
		},
		{
			name: "With failed system without error info",
			sys: *testutils.NewSystem(func(s *model.System) {
				s.Status = cmkapi.SystemStatusFAILED
			}),
			updateExpected: func(s model.System, sApi *cmkapi.System) {
				sApi.Metadata = &cmkapi.SystemMetadata{
					ErrorCode:    ptr.PointTo(constants.DefaultErrorCode),
					ErrorMessage: ptr.PointTo(constants.DefaultErrorMessage),
				}
			},
		},
		{
			name: "With failed system with error info",
			sys: *testutils.NewSystem(func(s *model.System) {
				s.Status = cmkapi.SystemStatusFAILED
				s.ErrorCode = uuid.NewString()
				s.ErrorMessage = uuid.NewString()
			}),
			updateExpected: func(s model.System, sApi *cmkapi.System) {
				sApi.Metadata = &cmkapi.SystemMetadata{
					ErrorCode:    ptr.PointTo(s.ErrorCode),
					ErrorMessage: ptr.PointTo(s.ErrorMessage),
				}
			},
		},
		{
			name: "With non-failed system with error info",
			sys: *testutils.NewSystem(func(s *model.System) {
				s.Status = cmkapi.SystemStatusCONNECTED
				s.ErrorCode = uuid.NewString()
				s.ErrorMessage = uuid.NewString()
			}),
			updateExpected: func(s model.System, sApi *cmkapi.System) {},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := system.ToAPI(tt.sys, tt.sysCfg)
			assert.NoError(t, err)
			assert.NotNil(t, res)
			var properties map[string]any
			expected := &cmkapi.System{
				ID:                   &tt.sys.ID,
				Identifier:           &tt.sys.Identifier,
				KeyConfigurationID:   tt.sys.KeyConfigurationID,
				KeyConfigurationName: tt.sys.KeyConfigurationName,
				Region:               tt.sys.Region,
				Status:               tt.sys.Status,
				Type:                 tt.sys.Type,
				Properties:           ptr.PointTo(properties),
			}
			tt.updateExpected(tt.sys, expected)
			assert.Equal(t, expected, res)
		})
	}
}

func TestFromAPIPatch(t *testing.T) {
	id := uuid.New()
	apiPatch := cmkapi.SystemPatch{
		KeyConfigurationID: id,
	}
	expected := model.System{
		KeyConfigurationID: &id,
	}

	assert.Equal(t, *expected.KeyConfigurationID, apiPatch.KeyConfigurationID)
}
