package system_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/api/transform/system"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/utils/ptr"
)

func TestToAPI(t *testing.T) {
	u := uuid.New()
	sys := model.System{
		ID:         u,
		Identifier: "givenSystem-001",
		Region:     "us-east-1",
		Status:     cmkapi.SystemStatusDISCONNECTED,
		Properties: make(map[string]string),
	}

	t.Run("With Properties", func(t *testing.T) {
		sys.Properties = map[string]string{"test": "test"}

		cfg := &config.System{OptionalProperties: map[string]config.SystemProperty{"test": {DisplayName: "test-display"}}}
		apiSys, err := system.ToAPI(sys, cfg)
		assert.NoError(t, err)
		assert.NotNil(t, apiSys)

		expectedSystem := cmkapi.System{
			ID:         &u,
			Identifier: ptr.PointTo("givenSystem-001"),
			Region:     "us-east-1",
			Status:     "DISCONNECTED",
			Properties: &map[string]any{"test": "test"},
			Metadata: &cmkapi.SystemMetadata{
				CanCancel: ptr.PointTo(false),
			},
		}

		assert.Equal(t, expectedSystem, *apiSys)
	})

	t.Run("Without Properties", func(t *testing.T) {
		cfg := &config.System{}
		apiSys, err := system.ToAPI(sys, cfg)
		assert.NoError(t, err)
		assert.NotNil(t, apiSys)

		expectedSystem := cmkapi.System{
			ID:         &u,
			Identifier: ptr.PointTo("givenSystem-001"),
			Region:     "us-east-1",
			Status:     "DISCONNECTED",
			Properties: &map[string]any{},
			Metadata: &cmkapi.SystemMetadata{
				CanCancel: ptr.PointTo(false),
			},
		}

		assert.Equal(t, expectedSystem, *apiSys)
	})
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
