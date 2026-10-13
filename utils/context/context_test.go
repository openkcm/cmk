package context_test

import (
	"context"
	"errors"
	"testing"

	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/constants"
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
	t.Run(
		"Should add tenant key to context", func(t *testing.T) {
			expected := "test"
			ctx := cmkcontext.New(context.TODO(), cmkcontext.WithTenant(expected))
			tenant, err := cmkcontext.ExtractTenantID(ctx)
			assert.NoError(t, err)
			assert.Equal(t, expected, tenant)
		},
	)
}

func TestExtractBusinessUserData(t *testing.T) {
	t.Run(
		"Should return error if no client data in context", func(t *testing.T) {
			_, err := cmkcontext.ExtractBusinessUserData(context.TODO())
			assert.ErrorIs(t, err, cmkcontext.ErrExtractBusinessUserData)
		},
	)

	t.Run(
		"Should extract client data from context", func(t *testing.T) {
			expected := &auth.ClientData{
				Identifier: "identifier",
				Email:      "email",
				Groups:     []string{"group1", "group2"},
				Region:     "region",
				Type:       "type",
			}
			ctx := context.WithValue(context.TODO(), constants.BusinessUserData, expected)
			businessUserData, err := cmkcontext.ExtractBusinessUserData(ctx)
			assert.NoError(t, err)
			assert.Equal(t, expected, businessUserData)
		},
	)
}

func TestExtractBusinessUserDataIdentifier(t *testing.T) {
	t.Run(
		"Should return error if no client data in context", func(t *testing.T) {
			_, err := cmkcontext.ExtractBusinessUserDataIdentifier(context.TODO())
			assert.ErrorIs(t, err, cmkcontext.ErrExtractBusinessUserData)
		},
	)

	t.Run(
		"Should extract client data identifier from context", func(t *testing.T) {
			expected := "identifier"
			businessUserData := &auth.ClientData{
				Identifier: expected,
				Email:      "email",
				Groups:     []string{"group1", "group2"},
				Region:     "region",
				Type:       "type",
			}
			ctx := cmkcontext.New(context.TODO(), cmkcontext.WithInjectBusinessUserData(businessUserData, nil))
			identifier, err := cmkcontext.ExtractBusinessUserDataIdentifier(ctx)
			assert.NoError(t, err)
			assert.Equal(t, expected, identifier)
		},
	)
}

func TestExtractBusinessUserDataGroups(t *testing.T) {
	t.Run(
		"Should return error if no client data in context", func(t *testing.T) {
			_, err := cmkcontext.ExtractBusinessUserDataGroups(context.TODO())
			assert.ErrorIs(t, err, cmkcontext.ErrExtractBusinessUserData)
		},
	)

	t.Run(
		"Should extract client data groups from context", func(t *testing.T) {
			expected := []string{"group1", "group2"}

			businessUserData := &auth.ClientData{
				Identifier: "identifier",
				Email:      "email",
				Groups:     []string{"group1", "group2"},
				Region:     "region",
				Type:       "type",
			}
			ctx := cmkcontext.New(context.TODO(), cmkcontext.WithInjectBusinessUserData(businessUserData, nil))
			groups, err := cmkcontext.ExtractBusinessUserDataGroups(ctx)
			assert.NoError(t, err)
			assert.Equal(t, expected, groups)
		},
	)
}

func TestExtractBusinessUserDataIssuer(t *testing.T) {
	t.Run(
		"Should return error if no client data in context", func(t *testing.T) {
			_, err := cmkcontext.ExtractBusinessUserDataIssuer(context.TODO())
			assert.ErrorIs(t, err, cmkcontext.ErrExtractBusinessUserDataAuthContext)
		},
	)

	t.Run(
		"Should extract client data issuer from context", func(t *testing.T) {
			expected := "issuer"
			businessUserData := &auth.ClientData{
				Identifier:  "identifier",
				Email:       "email",
				Groups:      []string{"group1", "group2"},
				Region:      "region",
				Type:        "type",
				AuthContext: map[string]string{"issuer": expected},
			}
			ctx := cmkcontext.New(context.TODO(), cmkcontext.WithInjectBusinessUserData(businessUserData, []string{"issuer"}))
			issuer, err := cmkcontext.ExtractBusinessUserDataIssuer(ctx)
			assert.NoError(t, err)
			assert.Equal(t, expected, issuer)
		},
	)
}

func TestExtractBusinessUserDataAuthContextField(t *testing.T) {
	t.Run(
		"Should return error if no client data in context", func(t *testing.T) {
			_, err := cmkcontext.ExtractBusinessUserDataAuthContextField(context.TODO(), "issuer")
			assert.ErrorIs(t, err, cmkcontext.ErrExtractBusinessUserDataAuthContext)
		},
	)

	t.Run(
		"Should return error if field not found in auth context", func(t *testing.T) {
			businessUserData := &auth.ClientData{
				Identifier:  "identifier",
				Email:       "email",
				Groups:      []string{"group1", "group2"},
				Region:      "region",
				Type:        "type",
				AuthContext: map[string]string{"foo": "bar"},
			}
			ctx := cmkcontext.New(context.TODO(), cmkcontext.WithInjectBusinessUserData(businessUserData, []string{"issuer"}))
			_, err := cmkcontext.ExtractBusinessUserDataAuthContextField(ctx, "issuer")
			assert.ErrorIs(t, err, cmkcontext.ErrExtractBusinessUserDataAuthContext)
		},
	)

	t.Run(
		"Should return error if field value is empty", func(t *testing.T) {
			businessUserData := &auth.ClientData{
				Identifier:  "identifier",
				Email:       "email",
				Groups:      []string{"group1", "group2"},
				Region:      "region",
				Type:        "type",
				AuthContext: map[string]string{"issuer": ""},
			}
			ctx := cmkcontext.New(context.TODO(), cmkcontext.WithInjectBusinessUserData(businessUserData, nil))
			_, err := cmkcontext.ExtractBusinessUserDataAuthContextField(ctx, "issuer")
			assert.ErrorIs(t, err, cmkcontext.ErrExtractBusinessUserDataAuthContext)
		},
	)

	t.Run(
		"Should extract specific field from client data auth context", func(t *testing.T) {
			expectedIssuer := "test-issuer"
			expectedAudience := "test-audience"
			businessUserData := &auth.ClientData{
				Identifier: "identifier",
				Email:      "email",
				Groups:     []string{"group1", "group2"},
				Region:     "region",
				Type:       "type",
				AuthContext: map[string]string{
					"issuer":   expectedIssuer,
					"audience": expectedAudience,
					"foo":      "bar",
				},
			}
			ctx := cmkcontext.New(context.TODO(),
				cmkcontext.WithInjectBusinessUserData(businessUserData, []string{"issuer", "audience"}))

			issuer, err := cmkcontext.ExtractBusinessUserDataAuthContextField(ctx, "issuer")
			assert.NoError(t, err)
			assert.Equal(t, expectedIssuer, issuer)

			audience, err := cmkcontext.ExtractBusinessUserDataAuthContextField(ctx, "audience")
			assert.NoError(t, err)
			assert.Equal(t, expectedAudience, audience)
		},
	)
}

func TestExtractBusinessUserDataAuthContext(t *testing.T) {
	t.Run(
		"Should return error if no client data in context", func(t *testing.T) {
			_, err := cmkcontext.ExtractBusinessUserDataAuthContext(context.TODO())
			assert.ErrorIs(t, err, cmkcontext.ErrExtractBusinessUserData)
		},
	)

	t.Run(
		"Should extract client data AuthContext from context", func(t *testing.T) {
			expected := map[string]string{"issuer": "issuer", "foo": "bar"}
			businessUserData := &auth.ClientData{
				Identifier:  "identifier",
				Email:       "email",
				Groups:      []string{"group1", "group2"},
				Region:      "region",
				Type:        "type",
				AuthContext: expected,
			}
			ctx := cmkcontext.New(context.TODO(),
				cmkcontext.WithInjectBusinessUserData(businessUserData, []string{"issuer", "foo"}))
			authContext, err := cmkcontext.ExtractBusinessUserDataAuthContext(ctx)
			assert.NoError(t, err)
			assert.Equal(t, expected, authContext)
		},
	)
}
