package tenantconfigs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/api/cmk/transform/tenantconfigs"
)

func TestLimitsToAPI(t *testing.T) {
	result := tenantconfigs.LimitsToAPI(50, 10, 5)

	require.NotNil(t, result)
	require.NotNil(t, result.Systems)
	assert.Equal(t, 50, *result.Systems)
	require.NotNil(t, result.Keys)
	assert.Equal(t, 10, *result.Keys)
	require.NotNil(t, result.KeyConfigurations)
	assert.Equal(t, 5, *result.KeyConfigurations)
}
