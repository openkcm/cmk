package context_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func TestExtractTenantID(t *testing.T) {
	tests := []struct {
		name      string
		tenantID  string
		want      string
		expectErr bool
	}{
		{
			name:      "Valid Tenant ID",
			tenantID:  "tenant123",
			want:      "tenant123",
			expectErr: false,
		},
		{
			name:      "Empty Tenant ID",
			tenantID:  "",
			want:      "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				ctx := testutils.CreateCtxWithTenant(tt.tenantID)

				got, err := cmkcontext.ExtractTenantID(ctx)
				if (err != nil) != tt.expectErr {
					t.Errorf("ExtractTenantID() error = %v, expectErr %v", err, tt.expectErr)
					return
				}

				if got != tt.want {
					t.Errorf("ExtractTenantID() = %v, want %v", got, tt.want)
				}

				if tt.expectErr && !errors.Is(err, cmkcontext.ErrExtractTenantID) {
					t.Errorf("Expected error to wrap ErrExtractTenantID, got %v", err)
				}
			},
		)
	}
}

func TestCreateTenantCtx(t *testing.T) {
	t.Run("Should add tenant key to context", func(t *testing.T) {
		expected := "test"
		ctx := cmkcontext.CreateTenantContext(context.TODO(), expected)
		tenant, err := cmkcontext.ExtractTenantID(ctx)
		assert.NoError(t, err)
		assert.Equal(t, expected, tenant)
	})
}
