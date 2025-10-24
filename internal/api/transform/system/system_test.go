package system_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/api/transform/system"
	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/utils/ptr"
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
			Properties: &map[string]interface{}{"test": "test"},
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
			Properties: &map[string]interface{}{},
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
