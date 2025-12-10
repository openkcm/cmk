package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/model"
)

func TestRoleValidation(t *testing.T) {
	tests := map[string]struct {
		role      model.TenantRole
		expectErr bool
	}{
		"Valid role": {
			role:      "ROLE_LIVE",
			expectErr: false,
		},
		"Empty role": {
			role:      "",
			expectErr: true,
		},
		"Unspecified role": {
			role:      "ROLE_UNSPECFIED",
			expectErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := test.role.Validate()
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
