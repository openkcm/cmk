package authz_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func TestNewRequest_ValidCases(t *testing.T) {
	tests := []struct {
		name         string
		user         authz.User
		resourceType authz.ResourceTypeName
		action       authz.Action
		tenantID     authz.TenantID
	}{
		{
			"ValidRequest",
			authz.User{UserName: "test_user", Groups: []authz.UserGroup{"group1"}},
			authz.ResourceTypeKey,
			authz.ActionKeyRead,
			"tenant1",
		},
		{
			"EmptyResourceType",
			authz.User{UserName: "test_user", Groups: []authz.UserGroup{"group1"}},
			authz.ResourceTypeName(""), authz.ActionKeyRead,
			"tenant1",
		},
		{
			"EmptyAction",
			authz.User{UserName: "test_user", Groups: []authz.UserGroup{"group1"}},
			authz.ResourceTypeKey,
			authz.Action(""),
			"tenant1",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				ctx := cmkcontext.CreateTenantContext(cmkcontext.InjectRequestID(t.Context()), string(tt.tenantID))

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
		resourceType authz.ResourceTypeName
		action       authz.Action
		tenantID     authz.TenantID
	}{
		{
			"EmptyUser",
			authz.User{UserName: "", Groups: []authz.UserGroup{"group1"}},
			authz.ResourceTypeKey,
			authz.ActionKeyRead,
			"tenant1",
		},
		{
			"InvalidResourceType",
			authz.User{UserName: "test_user", Groups: []authz.UserGroup{"group1"}},
			authz.ResourceTypeName("invalid"), authz.ActionKeyRead,
			"tenant1",
		},
		{
			"InvalidResourceTypeForAction",
			authz.User{UserName: "test_user", Groups: []authz.UserGroup{"group1"}},
			authz.ResourceTypeKeyConfiguration,
			authz.ActionKeyRead,
			"tenant1",
		},
		{
			"InvalidAction",
			authz.User{UserName: "test_user", Groups: []authz.UserGroup{"group1"}},
			authz.ResourceTypeKey,
			authz.Action("invalid"),
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

func TestSetUser(t *testing.T) {
	tests := []struct {
		name        string
		user        authz.User
		expectError bool
	}{
		{"ValidUser", authz.User{UserName: "test_user", Groups: []authz.UserGroup{"group1"}}, false},
		{"EmptyUser", authz.User{UserName: "", Groups: []authz.UserGroup{"group1"}}, true},
		{"EmptyUserGroup", authz.User{UserName: "test_user", Groups: []authz.UserGroup{}}, true},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				req := &authz.Request{}

				err := req.SetUser(tt.user)
				if tt.expectError && err == nil {
					t.Fatalf("expected error, got nil")
				}

				if !tt.expectError && err != nil {
					t.Fatalf("expected no error, got %v", err)
				}

				if req.User.UserName != tt.user.UserName && !tt.expectError {
					t.Errorf("expected UserName %v, got %v", tt.user, req.User)
				}
			},
		)
	}
}

func TestSetResourceType(t *testing.T) {
	test := []struct {
		name         string
		resourceType authz.ResourceTypeName
		action       authz.Action
		expectError  bool
	}{
		{"ValidResourceType", authz.ResourceTypeKey, authz.Action(""), false},
		{"EmptyResourceType", authz.ResourceTypeName(""), authz.Action(""), false},
		{"InvalidResourceType", authz.ResourceTypeName("invalid"), authz.Action(""), true},
		{"ValidResourceTypeWithInvalidAction", authz.ResourceTypeKeyConfiguration, authz.ActionKeyRead, true},
	}

	for _, tt := range test {
		t.Run(
			tt.name, func(t *testing.T) {
				req := &authz.Request{}
				// Set the Action first
				errAction := req.SetAction(tt.action)
				if errAction != nil {
					t.Fatalf("expected no error, got %v", errAction)
				}

				errResource := req.SetResourceType(tt.resourceType)
				if tt.expectError && errResource == nil {
					t.Fatalf("expected error, got nil")
				}

				if !tt.expectError && errResource != nil {
					t.Fatalf("expected no error, got %v", errResource)
				}

				if req.ResourceTypeName != tt.resourceType && !tt.expectError {
					t.Errorf("expected ResourceTypeName %v, got %v", tt.resourceType, req.ResourceTypeName)
				}
			},
		)
	}
}

func TestSetAction(t *testing.T) {
	// Test SetAction only with Action but without resourceType
	tests := []struct {
		name        string
		action      authz.Action
		expectError bool
	}{
		{"ValidAction", authz.ActionKeyRead, false},
		{"EmptyAction", authz.Action(""), false},
		{"InvalidAction", authz.Action("invalid"), true},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				req := &authz.Request{}

				err := req.SetAction(tt.action)
				if tt.expectError && err == nil {
					t.Fatalf("expected error, got nil")
				}

				if !tt.expectError && err != nil {
					t.Fatalf("expected no error, got %v", err)
				}

				if req.Action != tt.action && !tt.expectError {
					t.Errorf("expected Action %v, got %v", tt.action, req.Action)
				}
			},
		)
	}
}
