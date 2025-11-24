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
			ctx := cmkcontext.CreateTenantContext(context.TODO(), expected)
			tenant, err := cmkcontext.ExtractTenantID(ctx)
			assert.NoError(t, err)
			assert.Equal(t, expected, tenant)
		},
	)
}

func TestExtractClientData(t *testing.T) {
	t.Run(
		"Should return error if no client data in context", func(t *testing.T) {
			_, err := cmkcontext.ExtractClientData(context.TODO())
			assert.ErrorIs(t, err, cmkcontext.ErrExtractClientData)
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
			ctx := context.WithValue(context.TODO(), constants.ClientData, expected)
			clientData, err := cmkcontext.ExtractClientData(ctx)
			assert.NoError(t, err)
			assert.Equal(t, expected, clientData)
		},
	)
}

func TestExtractClientDataIdentifier(t *testing.T) {
	t.Run(
		"Should return error if no client data in context", func(t *testing.T) {
			_, err := cmkcontext.ExtractClientDataIdentifier(context.TODO())
			assert.ErrorIs(t, err, cmkcontext.ErrExtractClientData)
		},
	)

	t.Run(
		"Should extract client data identifier from context", func(t *testing.T) {
			expected := "identifier"
			clientData := &auth.ClientData{
				Identifier: expected,
				Email:      "email",
				Groups:     []string{"group1", "group2"},
				Region:     "region",
				Type:       "type",
			}
			ctx := context.WithValue(context.TODO(), constants.ClientData, clientData)
			identifier, err := cmkcontext.ExtractClientDataIdentifier(ctx)
			assert.NoError(t, err)
			assert.Equal(t, expected, identifier)
		},
	)
}

func TestExtractClientDataGroups(t *testing.T) {
	t.Run(
		"Should return error if no client data in context", func(t *testing.T) {
			_, err := cmkcontext.ExtractClientDataGroups(context.TODO())
			assert.ErrorIs(t, err, cmkcontext.ErrExtractClientData)
		},
	)

	t.Run(
		"Should extract client data groups from context", func(t *testing.T) {
			expected := []constants.UserGroup{"group1", "group2"}

			clientData := &auth.ClientData{
				Identifier: "identifier",
				Email:      "email",
				Groups:     []string{"group1", "group2"},
				Region:     "region",
				Type:       "type",
			}
			ctx := context.WithValue(context.TODO(), constants.ClientData, clientData)
			groups, err := cmkcontext.ExtractClientDataGroups(ctx)
			assert.NoError(t, err)
			assert.Equal(t, expected, groups)
		},
	)
}

func TestExtractClientDataIssuer(t *testing.T) {
	t.Run(
		"Should return error if no client data in context", func(t *testing.T) {
			_, err := cmkcontext.ExtractClientDataIssuer(context.TODO())
			assert.ErrorIs(t, err, cmkcontext.ErrExtractClientDataAuthContext)
		},
	)

	t.Run(
		"Should extract client data issuer from context", func(t *testing.T) {
			expected := "issuer"
			clientData := &auth.ClientData{
				Identifier:  "identifier",
				Email:       "email",
				Groups:      []string{"group1", "group2"},
				Region:      "region",
				Type:        "type",
				AuthContext: map[string]string{"issuer": expected},
			}
			ctx := context.WithValue(context.TODO(), constants.ClientData, clientData)
			issuer, err := cmkcontext.ExtractClientDataIssuer(ctx)
			assert.NoError(t, err)
			assert.Equal(t, expected, issuer)
		},
	)
}

func TestExtractClientDataAuthContextField(t *testing.T) {
	t.Run(
		"Should return error if no client data in context", func(t *testing.T) {
			_, err := cmkcontext.ExtractClientDataAuthContextField(context.TODO(), "issuer")
			assert.ErrorIs(t, err, cmkcontext.ErrExtractClientDataAuthContext)
		},
	)

	t.Run(
		"Should return error if field not found in auth context", func(t *testing.T) {
			clientData := &auth.ClientData{
				Identifier:  "identifier",
				Email:       "email",
				Groups:      []string{"group1", "group2"},
				Region:      "region",
				Type:        "type",
				AuthContext: map[string]string{"foo": "bar"},
			}
			ctx := context.WithValue(context.TODO(), constants.ClientData, clientData)
			_, err := cmkcontext.ExtractClientDataAuthContextField(ctx, "issuer")
			assert.ErrorIs(t, err, cmkcontext.ErrExtractClientDataAuthContext)
		},
	)

	t.Run(
		"Should return error if field value is empty", func(t *testing.T) {
			clientData := &auth.ClientData{
				Identifier:  "identifier",
				Email:       "email",
				Groups:      []string{"group1", "group2"},
				Region:      "region",
				Type:        "type",
				AuthContext: map[string]string{"issuer": ""},
			}
			ctx := context.WithValue(context.TODO(), constants.ClientData, clientData)
			_, err := cmkcontext.ExtractClientDataAuthContextField(ctx, "issuer")
			assert.ErrorIs(t, err, cmkcontext.ErrExtractClientDataAuthContext)
		},
	)

	t.Run(
		"Should extract specific field from client data auth context", func(t *testing.T) {
			expectedIssuer := "test-issuer"
			expectedAudience := "test-audience"
			clientData := &auth.ClientData{
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
			ctx := context.WithValue(context.TODO(), constants.ClientData, clientData)

			issuer, err := cmkcontext.ExtractClientDataAuthContextField(ctx, "issuer")
			assert.NoError(t, err)
			assert.Equal(t, expectedIssuer, issuer)

			audience, err := cmkcontext.ExtractClientDataAuthContextField(ctx, "audience")
			assert.NoError(t, err)
			assert.Equal(t, expectedAudience, audience)
		},
	)
}

func TestExtractClientDataAuthContext(t *testing.T) {
	t.Run(
		"Should return error if no client data in context", func(t *testing.T) {
			_, err := cmkcontext.ExtractClientDataAuthContext(context.TODO())
			assert.ErrorIs(t, err, cmkcontext.ErrExtractClientData)
		},
	)

	t.Run(
		"Should extract client data AuthContext from context", func(t *testing.T) {
			expected := map[string]string{"issuer": "issuer", "foo": "bar"}
			clientData := &auth.ClientData{
				Identifier:  "identifier",
				Email:       "email",
				Groups:      []string{"group1", "group2"},
				Region:      "region",
				Type:        "type",
				AuthContext: expected,
			}
			ctx := context.WithValue(context.TODO(), constants.ClientData, clientData)
			authContext, err := cmkcontext.ExtractClientDataAuthContext(ctx)
			assert.NoError(t, err)
			assert.Equal(t, expected, authContext)
		},
	)
}
