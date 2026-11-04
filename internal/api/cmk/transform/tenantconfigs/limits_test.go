package tenantconfigs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/api/cmk/transform/tenantconfigs"
	"github.com/openkcm/cmk/internal/model"
)

func TestLimitsToAPI(t *testing.T) {
	result := tenantconfigs.LimitsToAPI(model.TenantLimits{
		Systems:    new(50),
		Keys:       new(10),
		KeyConfigs: new(5),
	})

	require.NotNil(t, result)
	require.NotNil(t, result.Systems)
	assert.Equal(t, 50, *result.Systems)
	require.NotNil(t, result.Keys)
	assert.Equal(t, 10, *result.Keys)
	require.NotNil(t, result.KeyConfigurations)
	assert.Equal(t, 5, *result.KeyConfigurations)
}
