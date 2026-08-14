package namespace_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/multitenancy/namespace"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		tenantName string
		wantErr    bool
	}{
		{"valid_name", false},
		{"_validName123", false},
		{"in", true},         // too short
		{"pg_invalid", true}, // reserved prefix
	}

	for _, tt := range tests {
		t.Run(tt.tenantName, func(t *testing.T) {
			err := namespace.Validate(tt.tenantName)
			if tt.wantErr {
				require.Error(t, err, "Validate() should fail")
			} else {
				require.NoError(t, err, "Validate() should not fail")
			}
		})
	}
}
