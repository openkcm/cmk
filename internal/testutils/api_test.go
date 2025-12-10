package testutils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.tools.sap/kms/cmk/internal/daemon"
	"github.tools.sap/kms/cmk/internal/testutils"
)

// TestStartAPIServerReturnsServeMux tests if StartAPIServer returns a ServeMux
func TestStartAPIServerReturnsServeMux(t *testing.T) {
	db := &multitenancy.DB{}
	server := testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{
		Plugins: []testutils.MockPlugin{},
	})
	assert.NotNil(t, server)
	assert.IsType(t, &daemon.ServeMux{}, server)
}

func TestGetTestURL(t *testing.T) {
	tests := []struct {
		name     string
		tenant   string
		path     string
		expected string
	}{
		{
			name:     "Valid tenant and path",
			tenant:   "tenant123",
			path:     "resource",
			expected: "https://kms.test/cmk/v1/tenant123/resource",
		},
		{
			name:     "Empty tenant with path",
			tenant:   "",
			path:     "resource",
			expected: "https://kms.test/cmk/v1/test/resource",
		},
		{
			name:     "Path with trailing slash",
			tenant:   "tenant123",
			path:     "resource/",
			expected: "https://kms.test/cmk/v1/tenant123/resource/",
		},
		{
			name:     "Path with starting slash",
			tenant:   "tenant123",
			path:     "/resource",
			expected: "https://kms.test/cmk/v1/tenant123/resource",
		},
		{
			name:     "Empty tenant and empty path",
			tenant:   "",
			path:     "",
			expected: "https://kms.test/cmk/v1/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := testutils.GetTestURL(t, tt.tenant, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}
