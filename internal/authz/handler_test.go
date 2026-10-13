package authz_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/testutils"
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
				authHandler, err := authz.NewAuthorizationHandler(audit, APIInternalPolicies,
					authz.APIPolicies, authz.APIResourceTypeActions, &sync.Mutex{})
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
	authHandler, err := authz.NewAuthorizationHandler(audit,
		APIInternalPolicies, authz.APIPolicies, authz.APIResourceTypeActions, &sync.Mutex{})
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
