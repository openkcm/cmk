package authz_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

var EmptyTenantID = authz.TenantID("")

// TestIsAllowed tests the IsAllowed function of the AuthorizationHandler
func TestIsAllowed(t *testing.T) {
	tests := []struct {
		name               string
		entities           map[constants.BusinessRole]*authz.BusinessUser
		request            authz.Request[authz.BusinessUserRequest, authz.APIResourceType, authz.APIAction]
		expectError        bool
		expectedErrHandler bool
		expectAllow        bool
		tenantID           authz.TenantID
	}{
		{
			name: "NoExistentRole",
			entities: map[constants.BusinessRole]*authz.BusinessUser{
				"NonExistentRole": {
					TenantID: "tenant1",
					Groups:   []string{"Group1"},
				},
			},
			request: authz.Request[authz.BusinessUserRequest, authz.APIResourceType, authz.APIAction]{
				User: authz.BusinessUserRequest{
					TenantID: "tenant1",
					UserName: "test_user",
					Groups:   []string{"Group1"},
				},
				ResourceTypeName: authz.APIResourceTypeKey,
				Action:           authz.APIActionRead,
			},
			expectError:        true,
			expectedErrHandler: true,
			expectAllow:        false,
			tenantID:           "tenant1",
		},
		{
			name:     "EmptyEntities",
			entities: map[constants.BusinessRole]*authz.BusinessUser{},
			request: authz.Request[authz.BusinessUserRequest, authz.APIResourceType, authz.APIAction]{
				User: authz.BusinessUserRequest{
					TenantID: "tenant1",
					UserName: "test_user",
					Groups:   []string{"Group1"},
				},
				ResourceTypeName: authz.APIResourceTypeKey,
				Action:           authz.APIActionRead,
			},
			expectError:        true,
			expectedErrHandler: true,
			expectAllow:        false,
			tenantID:           "tenant1",
		},
		{
			name: "EmptyRequest",
			entities: map[constants.BusinessRole]*authz.BusinessUser{
				constants.TenantAdminRole: {
					TenantID: "tenant1",
					Groups:   []string{"Group1"},
				},
			},
			request: authz.Request[authz.BusinessUserRequest, authz.APIResourceType, authz.APIAction]{
				User: authz.BusinessUserRequest{
					TenantID: "tenant1",
					UserName: "test_user",
					Groups:   []string{"Group1"},
				},
				ResourceTypeName: authz.APIResourceType(""),
				Action:           authz.APIAction(""),
			},
			expectError:        true,
			expectedErrHandler: false,
			expectAllow:        false,
			tenantID:           "tenant1",
		},
		{
			name: "EmptyTenant",
			entities: map[constants.BusinessRole]*authz.BusinessUser{
				constants.KeyAdminRole: {
					TenantID: "tenant1",
					Groups:   []string{"Group1"},
				},
			},
			request: authz.Request[authz.BusinessUserRequest, authz.APIResourceType, authz.APIAction]{
				User: authz.BusinessUserRequest{
					TenantID: EmptyTenantID,
					UserName: "test_user",
					Groups:   []string{"Group1"},
				},
				ResourceTypeName: authz.APIResourceTypeKey,
				Action:           authz.APIActionRead,
			},
			expectError:        true,
			expectedErrHandler: false,
			expectAllow:        false,
			tenantID:           EmptyTenantID,
		},
		{
			name: "ValidRequestWithAllowedAction",
			entities: map[constants.BusinessRole]*authz.BusinessUser{
				constants.KeyAdminRole: {
					TenantID: "tenant1",
					Groups:   []string{"Group1"},
				},
			},
			request: authz.Request[authz.BusinessUserRequest, authz.APIResourceType, authz.APIAction]{
				User: authz.BusinessUserRequest{
					TenantID: "tenant1",
					UserName: "test_user",
					Groups:   []string{"Group1"},
				},
				ResourceTypeName: authz.APIResourceTypeKeyConfiguration,
				Action:           authz.APIActionRead,
			},
			expectError:        false,
			expectedErrHandler: false,
			expectAllow:        true,
			tenantID:           "tenant1",
		},
		{
			name: "ValidRequestWithNotAllowedAction",
			entities: map[constants.BusinessRole]*authz.BusinessUser{
				constants.KeyAdminRole: {
					TenantID: "tenant1",
					Groups:   []string{"Group1"},
				},
			},
			request: authz.Request[authz.BusinessUserRequest, authz.APIResourceType, authz.APIAction]{
				User: authz.BusinessUserRequest{
					TenantID: "tenant1",
					UserName: "test_user",
					Groups:   []string{"Group1"},
				},
				ResourceTypeName: authz.APIResourceTypeKeyConfiguration,
				Action:           authz.APIActionKeyRotate,
			},
			expectError:        true,
			expectedErrHandler: false,
			expectAllow:        false,
			tenantID:           "tenant1",
		},
	}

	cfg := &config.Config{}
	audit := auditor.New(context.Background(), cfg)

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				APIInternalPolicies := make(authz.RolePolicies[constants.InternalRole, authz.APIResourceType, authz.APIAction])
				APITechnicalPolicies := make(authz.RolePolicies[constants.TechnicalRole, authz.APIResourceType, authz.APIAction])
				authHandler, err := authz.NewAuthorizationHandler(audit, APIInternalPolicies,
					authz.APIPolicies, APITechnicalPolicies, authz.APIResourceTypeActions, &sync.Mutex{})
				assert.NoError(t, err)

				err = authHandler.UpdateBusinessUserData(tt.entities)
				if err != nil {
					if tt.expectedErrHandler {
						return
					}

					t.Fatalf("failed to create authorization handler: %v", err)

					return
				}

				ctx := testutils.CreateCtxWithTenant(string(tt.tenantID))
				ctx = context.WithValue(ctx, constants.UserType, constants.BusinessUser)

				decision, err := authHandler.IsBusinessUserAllowed(ctx, tt.request)
				if tt.expectError && err == nil {
					t.Fatalf("expected error, got nil")
				}

				if !tt.expectError && err != nil {
					t.Fatalf("expected no error, got %v", err)
				}

				if decision != tt.expectAllow {
					t.Errorf("expected decision %v, got %v", tt.expectAllow, decision)
				}
			},
		)
	}
}

const (
	totalNumber       = 10000
	testUsername      = "test_user"
	maxGroupCount     = 50
	entitiesPerTenant = 200
)

// BenchmarkIsAllowed benchmarks the IsAllowed function of the AuthorizationHandler
// It creates a large number of entities and runs the authorization check for different requests.

func BenchmarkIsAllowed(b *testing.B) {
	// Create test entities
	entities := createTestEntities(totalNumber)

	if entities == nil {
		b.Fatalf("Failed to create test entities")
	}

	cfg := &config.Config{}
	audit := auditor.New(context.Background(), cfg)

	// Initialize authorization handler
	APIInternalPolicies := make(authz.RolePolicies[constants.InternalRole,
		authz.APIResourceType, authz.APIAction])
	APITechnicalPolicies := make(authz.RolePolicies[constants.TechnicalRole,
		authz.APIResourceType, authz.APIAction])
	authHandler, err := authz.NewAuthorizationHandler(audit,
		APIInternalPolicies, authz.APIPolicies, APITechnicalPolicies, authz.APIResourceTypeActions, &sync.Mutex{})
	if err != nil {
		b.Fatalf("Failed to create authorization handler: %v", err)
	}

	err = authHandler.UpdateBusinessUserData(entities)
	if err != nil {
		b.Fatalf("Failed to create authorization handler: %v", err)
	}

	// test different requests
	request := []struct {
		name    string
		request authz.Request[authz.BusinessUserRequest,
			authz.APIResourceType, authz.APIAction]
		tenantID authz.TenantID
	}{
		{
			name: "singleGroupCommonAccess",
			request: authz.Request[authz.BusinessUserRequest,
				authz.APIResourceType, authz.APIAction]{
				User: authz.BusinessUserRequest{
					TenantID: "tenant2000",
					UserName: testUsername,
					Groups:   []string{"Group1"},
				},
				ResourceTypeName: authz.APIResourceTypeKeyConfiguration,
				Action:           authz.APIActionRead,
			},
			tenantID: authz.TenantID("tenant2000"),
		},
		{
			name: "multipleGroupsCommonAccess",
			request: authz.Request[authz.BusinessUserRequest,
				authz.APIResourceType, authz.APIAction]{
				User: authz.BusinessUserRequest{
					TenantID: "tenant300",
					UserName: testUsername,
					Groups:   []string{"Group1", "Group2"},
				},
				ResourceTypeName: authz.APIResourceTypeKeyConfiguration,
				Action:           authz.APIActionRead,
			},
			tenantID: authz.TenantID("tenant300"),
		},
		{
			name: "singleGroupNoAccess",
			request: authz.Request[authz.BusinessUserRequest,
				authz.APIResourceType, authz.APIAction]{
				User: authz.BusinessUserRequest{
					TenantID: "tenant3",
					UserName: testUsername,
					Groups:   []string{"Groupxz"},
				},
				ResourceTypeName: authz.APIResourceTypeKeyConfiguration,
				Action:           authz.APIActionDelete,
			},
			tenantID: authz.TenantID("tenant3"),
		},
	}

	for _, req := range request {
		b.Run(
			req.name, func(b *testing.B) {
				ctx := testutils.CreateCtxWithTenant(string(req.tenantID))

				// Run benchmark
				b.ResetTimer()

				for range b.N {
					_, _ = authHandler.IsBusinessUserAllowed(ctx, req.request)
				}
			},
		)
	}
}

// RoleAssignment handles the logic for assigning roles based on the entity index
type RoleAssignment struct {
	roles []constants.BusinessRole
}

func newRoleAssignment() *RoleAssignment {
	return &RoleAssignment{
		roles: []constants.BusinessRole{
			constants.TenantAuditorRole,
			constants.TenantAdminRole,
			constants.KeyAdminRole,
		},
	}
}

func (ra *RoleAssignment) getRoleForIndex(idx int) constants.BusinessRole {
	if len(ra.roles) == 0 || idx < 0 {
		return ""
	}

	return ra.roles[idx%len(ra.roles)]
}

func createTestEntities(totalCount int) map[constants.BusinessRole]*authz.BusinessUser {
	if totalCount%entitiesPerTenant != 0 {
		return nil
	}

	numTenants := totalCount / entitiesPerTenant
	entities := make(map[constants.BusinessRole]*authz.BusinessUser)

	userGroups := generateUserGroups()
	roleAssigner := newRoleAssignment()

	for tenantIdx := range numTenants {
		createEntitiesForTenant(tenantIdx, userGroups, roleAssigner, entities)
	}

	return entities
}

func generateUserGroups() []string {
	groups := make([]string, maxGroupCount)

	for i := range maxGroupCount {
		groups[i] = fmt.Sprintf("Group%d", i+1)
	}

	return groups
}

func newTestHandler(t *testing.T) *authz.Handler[authz.APIResourceType, authz.APIAction] {
	t.Helper()
	cfg := &config.Config{}
	audit := auditor.New(context.Background(), cfg)
	internalPolicies := make(authz.RolePolicies[constants.InternalRole, authz.APIResourceType, authz.APIAction])
	technicalPolicies := make(authz.RolePolicies[constants.TechnicalRole, authz.APIResourceType, authz.APIAction])
	h, err := authz.NewAuthorizationHandler(audit, internalPolicies, authz.APIPolicies, technicalPolicies, authz.APIResourceTypeActions, &sync.Mutex{})
	assert.NoError(t, err)
	return h
}

func businessCtx(tenant, identifier string, groups []string) context.Context {
	ctx := cmkcontext.CreateTenantContext(context.Background(), tenant)
	ctx = cmkcontext.InjectRequestID(ctx, "req-id")
	userData := &auth.ClientData{Identifier: identifier, Groups: groups}
	return cmkcontext.InjectBusinessUserData(ctx, userData, nil)
}

func TestAuditDeniedDecision(t *testing.T) {
	h := newTestHandler(t)

	tests := []struct {
		name    string
		makeCtx func() context.Context
	}{
		{
			name:    "no user type",
			makeCtx: func() context.Context { return testutils.CreateCtxWithTenant("tenant1") },
		},
		{
			name: "internal user type",
			makeCtx: func() context.Context {
				ctx, _ := cmkcontext.InjectInternalUserData(
					testutils.CreateCtxWithTenant("tenant1"),
					constants.InternalRole("some-role"),
				)
				return ctx
			},
		},
		{
			name: "missing tenant",
			makeCtx: func() context.Context {
				return context.WithValue(context.Background(), constants.UserType, constants.BusinessUser)
			},
		},
		{
			name: "missing business user data",
			makeCtx: func() context.Context {
				ctx := testutils.CreateCtxWithTenant("tenant1")
				return context.WithValue(ctx, constants.UserType, constants.BusinessUser)
			},
		},
		{
			name: "missing request ID",
			makeCtx: func() context.Context {
				// InjectBusinessUserData sets UserType + BusinessUserData but no request ID
				ctx := cmkcontext.CreateTenantContext(context.Background(), "tenant1")
				userData := &auth.ClientData{Identifier: "user1", Groups: []string{"g1"}}
				return cmkcontext.InjectBusinessUserData(ctx, userData, nil)
			},
		},
		{
			name:    "valid context",
			makeCtx: func() context.Context { return businessCtx("tenant1", "user1", []string{"g1"}) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// AuditDeniedDecision must not panic regardless of context state
			assert.NotPanics(t, func() {
				h.AuditDeniedDecision(tt.makeCtx(), authz.APIResourceTypeKey, authz.APIActionRead)
			})
		})
	}
}

func TestIsTechnicalUserAllowed(t *testing.T) {
	const (
		tenantID    = authz.TenantID("tenant1")
		techRole    = constants.TechnicalRole("TECH_ROLE")
		otherTenant = authz.TenantID("other-tenant")
	)

	technicalPolicies := authz.RolePolicies[constants.TechnicalRole, authz.APIResourceType, authz.APIAction]{
		techRole: {
			{
				ID: "TechPolicy",
				ResourceTypes: []authz.Resource[authz.APIResourceType, authz.APIAction]{
					{
						Type:    authz.APIResourceTypeKey,
						Actions: []authz.APIAction{authz.APIActionRead},
					},
				},
			},
		},
	}

	newHandler := func() *authz.Handler[authz.APIResourceType, authz.APIAction] {
		cfg := &config.Config{}
		audit := auditor.New(context.Background(), cfg)
		internalPolicies := make(authz.RolePolicies[constants.InternalRole, authz.APIResourceType, authz.APIAction])
		h, err := authz.NewAuthorizationHandler(audit, internalPolicies, authz.APIPolicies, technicalPolicies, authz.APIResourceTypeActions, &sync.Mutex{})
		assert.NoError(t, err)
		h.UpdateTechnicalUserData(tenantID)
		return h
	}

	technicalCtx := func(tid authz.TenantID, role constants.TechnicalRole) context.Context {
		ctx := cmkcontext.CreateTenantContext(context.Background(), string(tid))
		ctx = cmkcontext.InjectRequestID(ctx, "req-id")
		ctx, _ = cmkcontext.InjectTechnicalUserData(ctx, role)
		return ctx
	}

	tests := []struct {
		name        string
		makeCtx     func() context.Context
		request     authz.Request[authz.TechnicalUserRequest, authz.APIResourceType, authz.APIAction]
		expectAllow bool
		expectErr   bool
	}{
		{
			name:    "allowed",
			makeCtx: func() context.Context { return technicalCtx(tenantID, techRole) },
			request: authz.Request[authz.TechnicalUserRequest, authz.APIResourceType, authz.APIAction]{
				User:             authz.TechnicalUserRequest{TenantID: tenantID, Role: techRole},
				ResourceTypeName: authz.APIResourceTypeKey,
				Action:           authz.APIActionRead,
			},
			expectAllow: true,
		},
		{
			name:    "denied - action not in policy",
			makeCtx: func() context.Context { return technicalCtx(tenantID, techRole) },
			request: authz.Request[authz.TechnicalUserRequest, authz.APIResourceType, authz.APIAction]{
				User:             authz.TechnicalUserRequest{TenantID: tenantID, Role: techRole},
				ResourceTypeName: authz.APIResourceTypeKey,
				Action:           authz.APIActionCreate,
			},
			expectAllow: false,
			expectErr:   true,
		},
		{
			name:    "denied - unknown role",
			makeCtx: func() context.Context { return technicalCtx(tenantID, "UNKNOWN_ROLE") },
			request: authz.Request[authz.TechnicalUserRequest, authz.APIResourceType, authz.APIAction]{
				User:             authz.TechnicalUserRequest{TenantID: tenantID, Role: "UNKNOWN_ROLE"},
				ResourceTypeName: authz.APIResourceTypeKey,
				Action:           authz.APIActionRead,
			},
			expectAllow: false,
			expectErr:   true,
		},
		{
			name:    "denied - tenant not loaded",
			makeCtx: func() context.Context { return technicalCtx(otherTenant, techRole) },
			request: authz.Request[authz.TechnicalUserRequest, authz.APIResourceType, authz.APIAction]{
				User:             authz.TechnicalUserRequest{TenantID: otherTenant, Role: techRole},
				ResourceTypeName: authz.APIResourceTypeKey,
				Action:           authz.APIActionRead,
			},
			expectAllow: false,
			expectErr:   true,
		},
		{
			name:    "error - tenant mismatch in context",
			makeCtx: func() context.Context { return technicalCtx("wrong-tenant", techRole) },
			request: authz.Request[authz.TechnicalUserRequest, authz.APIResourceType, authz.APIAction]{
				User:             authz.TechnicalUserRequest{TenantID: tenantID, Role: techRole},
				ResourceTypeName: authz.APIResourceTypeKey,
				Action:           authz.APIActionRead,
			},
			expectAllow: false,
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHandler()
			allowed, err := h.IsTechnicalUserAllowed(tt.makeCtx(), tt.request)
			assert.Equal(t, tt.expectAllow, allowed)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateAndResetTechnicalUserData(t *testing.T) {
	const (
		tenantID = authz.TenantID("tenant1")
		techRole = constants.TechnicalRole("TECH_ROLE")
	)

	technicalPolicies := authz.RolePolicies[constants.TechnicalRole, authz.APIResourceType, authz.APIAction]{
		techRole: {
			{
				ID: "TechPolicy",
				ResourceTypes: []authz.Resource[authz.APIResourceType, authz.APIAction]{
					{Type: authz.APIResourceTypeKey, Actions: []authz.APIAction{authz.APIActionRead}},
				},
			},
		},
	}

	cfg := &config.Config{}
	audit := auditor.New(context.Background(), cfg)
	internalPolicies := make(authz.RolePolicies[constants.InternalRole, authz.APIResourceType, authz.APIAction])
	h, err := authz.NewAuthorizationHandler(audit, internalPolicies, authz.APIPolicies, technicalPolicies, authz.APIResourceTypeActions, &sync.Mutex{})
	assert.NoError(t, err)

	ctx := cmkcontext.CreateTenantContext(context.Background(), string(tenantID))
	ctx = cmkcontext.InjectRequestID(ctx, "req-id")
	ctx, _ = cmkcontext.InjectTechnicalUserData(ctx, techRole)
	req := authz.Request[authz.TechnicalUserRequest, authz.APIResourceType, authz.APIAction]{
		User:             authz.TechnicalUserRequest{TenantID: tenantID, Role: techRole},
		ResourceTypeName: authz.APIResourceTypeKey,
		Action:           authz.APIActionRead,
	}

	// before loading tenant — denied
	_, err = h.IsTechnicalUserAllowed(ctx, req)
	assert.Error(t, err)

	// after loading tenant — allowed
	h.UpdateTechnicalUserData(tenantID)
	allowed, err := h.IsTechnicalUserAllowed(ctx, req)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// after reset — denied again
	h.ResetTechnicalUserData()
	_, err = h.IsTechnicalUserAllowed(ctx, req)
	assert.Error(t, err)
}

func createEntitiesForTenant(
	tenantIdx int, allGroups []string, roleAssigner *RoleAssignment, entities map[constants.BusinessRole]*authz.BusinessUser,
) {
	tenantID := authz.TenantID(fmt.Sprintf("tenant%d", tenantIdx+1))

	for entityIdx := range entitiesPerTenant {
		globalIdx := tenantIdx*entitiesPerTenant + entityIdx
		groupCount := (globalIdx % maxGroupCount) + 1

		// Ensure groupCount does not exceed the length of allGroups
		safeGroupCount := min(groupCount, len(allGroups))

		role := roleAssigner.getRoleForIndex(globalIdx)
		entities[role] = &authz.BusinessUser{
			TenantID: tenantID,
			Groups:   allGroups[:safeGroupCount],
		}
	}
}
