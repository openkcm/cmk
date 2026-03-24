package authz_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func TestNewRequest_ValidCases(t *testing.T) {
	tests := []struct {
		name         string
		user         authz.User
		resourceType authz.APIResourceTypeName
		action       authz.APIAction
		tenantID     authz.TenantID
	}{
		{
			"ValidRequest",
			authz.User{UserName: "test_user", Groups: []string{"group1"}},
			authz.APIResourceTypeKey,
			authz.APIActionRead,
			"tenant1",
		},
		{
			"EmptyAPIResourceType",
			authz.User{UserName: "test_user", Groups: []string{"group1"}},
			authz.APIResourceTypeName(""), authz.APIActionRead,
			"tenant1",
		},
		{
			"EmptyAPIAction",
			authz.User{UserName: "test_user", Groups: []string{"group1"}},
			authz.APIResourceTypeKey,
			authz.APIAction(""),
			"tenant1",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				ctx := cmkcontext.CreateTenantContext(cmkcontext.InjectRequestID(t.Context(), uuid.NewString()), string(tt.tenantID))

				req, err := authz.NewRequest(ctx, tt.tenantID, tt.user, tt.resourceType, tt.action)
				assert.NoError(t, err)
				assert.NotNil(t, req)
				assert.Equal(t, tt.user.UserName, req.User.UserName)
				assert.Equal(t, tt.resourceType, req.ResourceTypeName)
				assert.Equal(t, tt.action, req.Action)
			},
		)
	}
}

func TestNewRequest_InvalidCases(t *testing.T) {
	tests := []struct {
		name         string
		user         authz.User
		resourceType authz.APIResourceTypeName
		action       authz.APIAction
		tenantID     authz.TenantID
	}{
		{
			"EmptyUser",
			authz.User{UserName: "", Groups: []string{"group1"}},
			authz.APIResourceTypeKey,
			authz.APIActionRead,
			"tenant1",
		},
		{
			"EmptyUserGroups",
			authz.User{UserName: "test_user", Groups: []string{}},
			authz.APIResourceTypeKey,
			authz.APIActionRead,
			"tenant1",
		},
		{
			"InvalidAPIResourceType",
			authz.User{UserName: "test_user", Groups: []string{"group1"}},
			authz.APIResourceTypeName("invalid"), authz.APIActionRead,
			"tenant1",
		},
		{
			"InvalidAPIResourceTypeForAPIAction",
			authz.User{UserName: "test_user", Groups: []string{"group1"}},
			authz.APIResourceTypeKeyConfiguration,
			authz.APIActionRead,
			"tenant1",
		},
		{
			"InvalidAPIAction",
			authz.User{UserName: "test_user", Groups: []string{"group1"}},
			authz.APIResourceTypeKey,
			authz.APIAction("invalid"),
			"tenant1",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				ctx := testutils.CreateCtxWithTenant(string(tt.tenantID))

				req, err := authz.NewRequest(ctx, tt.tenantID, tt.user, tt.resourceType, tt.action)
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
