package authz_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func TestNewRequest_ValidCases(t *testing.T) {
	tests := []struct {
		name         string
		user         authz.BusinessUserRequest
		resourceType authz.APIResourceType
		action       authz.APIAction
	}{
		{
			"ValidRequest",
			authz.BusinessUserRequest{
				TenantID: "tenant1",
				UserName: "test_user", Groups: []string{"group1"},
			},
			authz.APIResourceTypeKey,
			authz.APIActionRead,
		},
		{
			"EmptyAPIResourceType",
			authz.BusinessUserRequest{
				TenantID: "tenant1",
				UserName: "test_user", Groups: []string{"group1"},
			},
			authz.APIResourceType(""), authz.APIActionRead,
		},
		{
			"EmptyAPIAction",
			authz.BusinessUserRequest{
				TenantID: "tenant1",
				UserName: "test_user", Groups: []string{"group1"},
			},
			authz.APIResourceTypeKey,
			authz.APIAction(""),
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				ctx := cmkcontext.CreateTenantContext(cmkcontext.InjectRequestID(t.Context(),
					uuid.NewString()), string(tt.user.TenantID))

				req, err := authz.NewRequest(ctx, tt.user, tt.resourceType, tt.action)
				assert.NoError(t, err)
				assert.NotNil(t, req)
				assert.Equal(t, tt.user.UserName, req.User.UserName)
				assert.Equal(t, tt.resourceType, req.ResourceTypeName)
				assert.Equal(t, tt.action, req.Action)
			},
		)
	}
}

func TestTechnicalUserRequest_IsValidContext(t *testing.T) {
	tests := []struct {
		name      string
		request   authz.TechnicalUserRequest
		setupCtx  func(tenantID string) context.Context
		expectErr error
	}{
		{
			name:    "valid",
			request: authz.TechnicalUserRequest{TenantID: "tenant1", Role: "tech-role"},
			setupCtx: func(tenantID string) context.Context {
				ctx := cmkcontext.CreateTenantContext(cmkcontext.InjectRequestID(t.Context(), uuid.NewString()), tenantID)
				ctx, _ = cmkcontext.InjectTechnicalUserData(ctx, "tech-role")
				return ctx
			},
		},
		{
			name:    "tenant mismatch",
			request: authz.TechnicalUserRequest{TenantID: "tenant1", Role: "tech-role"},
			setupCtx: func(_ string) context.Context {
				ctx := cmkcontext.CreateTenantContext(cmkcontext.InjectRequestID(t.Context(), uuid.NewString()), "other-tenant")
				ctx, _ = cmkcontext.InjectTechnicalUserData(ctx, "tech-role")
				return ctx
			},
			expectErr: authz.ErrWrongTenantID,
		},
		{
			name:    "wrong user type",
			request: authz.TechnicalUserRequest{TenantID: "tenant1", Role: "tech-role"},
			setupCtx: func(tenantID string) context.Context {
				ctx := cmkcontext.CreateTenantContext(cmkcontext.InjectRequestID(t.Context(), uuid.NewString()), tenantID)
				return context.WithValue(ctx, constants.UserType, constants.BusinessUser)
			},
			expectErr: authz.ErrNoAuthzForUserType,
		},
		{
			name:    "no tenant in context",
			request: authz.TechnicalUserRequest{TenantID: "tenant1", Role: "tech-role"},
			setupCtx: func(_ string) context.Context {
				return cmkcontext.InjectRequestID(t.Context(), uuid.NewString())
			},
			expectErr: cmkcontext.ErrExtractTenantID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx(string(tt.request.TenantID))
			err := tt.request.IsValidContext(ctx)
			if tt.expectErr != nil {
				assert.ErrorIs(t, err, tt.expectErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTechnicalUserRequest_IsEmpty(t *testing.T) {
	assert.True(t, authz.TechnicalUserRequest{}.IsEmpty())
	assert.False(t, authz.TechnicalUserRequest{Role: "tech-role"}.IsEmpty())
}

func TestTechnicalUserRequest_IsInternal(t *testing.T) {
	assert.False(t, authz.TechnicalUserRequest{Role: "tech-role"}.IsInternal())
}

func TestNewRequest_InvalidCases(t *testing.T) {
	tests := []struct {
		name         string
		user         authz.BusinessUserRequest
		resourceType authz.APIResourceType
		action       authz.APIAction
	}{
		{
			"EmptyUser",
			authz.BusinessUserRequest{
				TenantID: "tenant1",
				UserName: "", Groups: []string{"group1"},
			},
			authz.APIResourceTypeKey,
			authz.APIActionRead,
		},
		{
			"EmptyUserGroups",
			authz.BusinessUserRequest{
				TenantID: "tenant1",
				UserName: "test_user", Groups: []string{},
			},
			authz.APIResourceTypeKey,
			authz.APIActionRead,
		},
		{
			"InvalidAPIResourceType",
			authz.BusinessUserRequest{
				TenantID: "tenant1",
				UserName: "test_user", Groups: []string{"group1"},
			},
			authz.APIResourceType("invalid"), authz.APIActionRead,
		},
		{
			"InvalidAPIResourceTypeForAPIAction",
			authz.BusinessUserRequest{
				TenantID: "tenant1",
				UserName: "test_user", Groups: []string{"group1"},
			},
			authz.APIResourceTypeKeyConfiguration,
			authz.APIActionRead,
		},
		{
			"InvalidAPIAction",
			authz.BusinessUserRequest{
				TenantID: "tenant1",
				UserName: "test_user", Groups: []string{"group1"},
			},
			authz.APIResourceTypeKey,
			authz.APIAction("invalid"),
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				ctx := testutils.CreateCtxWithTenant(string(tt.user.TenantID))

				req, err := authz.NewRequest(ctx, tt.user, tt.resourceType, tt.action)
				if err == nil {
					t.Fatalf("expected error, got nil")
				}

				if req != nil {
					t.Fatalf("expected nil request, got %v", req)
				}
			},
		)
	}
}
