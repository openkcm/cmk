package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.tools.sap/kms/cmk/internal/model"
)

func TestTenantStatusValidation(t *testing.T) {
	tests := map[string]struct {
		status    model.TenantStatus
		expectErr bool
	}{
		"Valid status": {
			status:    model.TenantStatus(pb.Status_STATUS_ACTIVE.String()),
			expectErr: false,
		},
		"Empty status": {
			status:    "",
			expectErr: true,
		},
		"Invalid status": {
			status:    "invalid_status",
			expectErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := test.status.Validate()
			if test.expectErr {
				assert.Error(t, err)
				assert.Equal(t, model.ErrInvalidTenantStatus, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
