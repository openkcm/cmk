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
		user         authz.BusinessUserRequest
		resourceType authz.APIResourceTypeName
		action       authz.APIAction
	}{
		{
			"ValidRequest",
			authz.BusinessUserRequest{TenantID: "tenant1",
				UserName: "test_user", Groups: []string{"group1"}},
			authz.APIResourceTypeKey,
			authz.APIActionRead,
		},
		{
			"EmptyAPIResourceType",
			authz.BusinessUserRequest{TenantID: "tenant1",
				UserName: "test_user", Groups: []string{"group1"}},
			authz.APIResourceTypeName(""), authz.APIActionRead,
		},
		{
			"EmptyAPIAction",
			authz.BusinessUserRequest{TenantID: "tenant1",
				UserName: "test_user", Groups: []string{"group1"}},
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

func TestNewRequest_InvalidCases(t *testing.T) {
	tests := []struct {
		name         string
		user         authz.BusinessUserRequest
		resourceType authz.APIResourceTypeName
		action       authz.APIAction
	}{
		{
			"EmptyUser",
			authz.BusinessUserRequest{TenantID: "tenant1",
				UserName: "", Groups: []string{"group1"}},
			authz.APIResourceTypeKey,
			authz.APIActionRead,
		},
		{
			"EmptyUserGroups",
			authz.BusinessUserRequest{TenantID: "tenant1",
				UserName: "test_user", Groups: []string{}},
			authz.APIResourceTypeKey,
			authz.APIActionRead,
		},
		{
			"InvalidAPIResourceType",
			authz.BusinessUserRequest{TenantID: "tenant1",
				UserName: "test_user", Groups: []string{"group1"}},
			authz.APIResourceTypeName("invalid"), authz.APIActionRead,
		},
		{
			"InvalidAPIResourceTypeForAPIAction",
			authz.BusinessUserRequest{TenantID: "tenant1",
				UserName: "test_user", Groups: []string{"group1"}},
			authz.APIResourceTypeKeyConfiguration,
			authz.APIActionRead,
		},
		{
			"InvalidAPIAction",
			authz.BusinessUserRequest{TenantID: "tenant1",
				UserName: "test_user", Groups: []string{"group1"}},
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
