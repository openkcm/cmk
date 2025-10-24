package authz_test

import (
	"fmt"
	"testing"

	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/testutils"
)

// TestIsAllowed tests the IsAllowed function of the AuthorizationHandler
func TestIsAllowed(t *testing.T) {
	tests := []struct {
		name               string
		entities           []authz.Entity
		request            authz.Request
		expectError        bool
		expectedErrHandler bool
		expectAllow        bool
		tenantID           authz.TenantID
	}{
		{
			name: "NoExistentRole",
			entities: []authz.Entity{
				{TenantID: "tenant1", Role: "NonExistentRole", UserGroups: []authz.UserGroup{"Group1"}},
			},
			request: authz.Request{
				User:             authz.User{UserName: "test_user", Groups: []authz.UserGroup{"Group1"}},
				ResourceTypeName: authz.ResourceTypeKey,
				Action:           authz.ActionKeyRead,
				TenantID:         "tenant1",
			},
			expectError:        true,
			expectedErrHandler: true,
			expectAllow:        false,
			tenantID:           "tenant1",
		},
		{
			name:     "EmptyEntities",
			entities: []authz.Entity{},
			request: authz.Request{
				User:             authz.User{UserName: "test_user", Groups: []authz.UserGroup{"Group1"}},
				ResourceTypeName: authz.ResourceTypeKey,
				Action:           authz.ActionKeyRead,
				TenantID:         "tenant1",
			},
			expectError:        true,
			expectedErrHandler: true,
			expectAllow:        false,
			tenantID:           "tenant1",
		},
		{
			name: "EmptyRequest",
			entities: []authz.Entity{
				{TenantID: "tenant1", Role: constants.TenantAdminRole, UserGroups: []authz.UserGroup{"Group1"}},
			},
			request: authz.Request{
				User:             authz.User{UserName: "test_user", Groups: []authz.UserGroup{"Group1"}},
				ResourceTypeName: authz.ResourceTypeName(""),
				Action:           authz.Action(""),
				TenantID:         "tenant1",
			},
			expectError:        true,
			expectedErrHandler: false,
			expectAllow:        false,
			tenantID:           "tenant1",
		},
		{
			name: "EmptyTenant",
			entities: []authz.Entity{
				{TenantID: "tenant1", Role: constants.KeyAdminRole, UserGroups: []authz.UserGroup{"Group1"}},
			},
			request: authz.Request{
				User:             authz.User{UserName: "test_user", Groups: []authz.UserGroup{"Group1"}},
				ResourceTypeName: authz.ResourceTypeKey,
				Action:           authz.ActionKeyRead,
				TenantID:         authz.EmptyTenantID,
			},
			expectError:        true,
			expectedErrHandler: false,
			expectAllow:        false,
			tenantID:           authz.EmptyTenantID,
		},
		{
			name: "ValidRequestWithAllowedAction",
			entities: []authz.Entity{
				{TenantID: "tenant1", Role: constants.KeyAdminRole, UserGroups: []authz.UserGroup{"Group1"}},
			},
			request: authz.Request{
				User:             authz.User{UserName: "test_user", Groups: []authz.UserGroup{"Group1"}},
				ResourceTypeName: authz.ResourceTypeKeyConfiguration,
				Action:           authz.ActionKeyConfigurationRead,
				TenantID:         "tenant1",
			},
			expectError:        false,
			expectedErrHandler: false,
			expectAllow:        true,
			tenantID:           "tenant1",
		},
		{
			name: "ValidRequestWithNotAllowedAction",
			entities: []authz.Entity{
				{TenantID: "tenant1", Role: constants.KeyAdminRole, UserGroups: []authz.UserGroup{"Group1"}},
			},
			request: authz.Request{
				User:             authz.User{UserName: "test_user", Groups: []authz.UserGroup{"Group1"}},
				ResourceTypeName: authz.ResourceTypeKeyConfiguration,
				Action:           authz.ActionKeyRead,
				TenantID:         "tenant1",
			},
			expectError:        true,
			expectedErrHandler: false,
			expectAllow:        false,
			tenantID:           "tenant1",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				authHandler, err := authz.NewAuthorizationHandler(&tt.entities)
				if err != nil {
					if tt.expectedErrHandler {
						return
					}

					t.Fatalf("failed to create authorization handler: %v", err)

					return
				}

				ctx := testutils.CreateCtxWithTenant(string(tt.tenantID))

				decision, err := authHandler.IsAllowed(ctx, tt.request)
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

	// Initialize authorization handler
	authHandler, err := authz.NewAuthorizationHandler(&entities)
	if err != nil {
		b.Fatalf("Failed to create authorization handler: %v", err)
	}

	// test different requests
	request := []struct {
		name     string
		request  authz.Request
		tenantID authz.TenantID
	}{
		{
			name: "singleGroupCommonAccess",
			request: authz.Request{
				User:             authz.User{UserName: testUsername, Groups: []authz.UserGroup{"Group1"}},
				ResourceTypeName: authz.ResourceTypeKeyConfiguration,
				Action:           authz.ActionKeyConfigurationRead,
				TenantID:         "tenant2000",
			},
			tenantID: authz.TenantID("tenant2000"),
		},
		{
			name: "multipleGroupsCommonAccess",
			request: authz.Request{
				User:             authz.User{UserName: testUsername, Groups: []authz.UserGroup{"Group1", "Group2"}},
				ResourceTypeName: authz.ResourceTypeKeyConfiguration,
				Action:           authz.ActionKeyConfigurationRead,
				TenantID:         "tenant300",
			},
			tenantID: authz.TenantID("tenant300"),
		},
		{
			name: "singleGroupNoAccess",
			request: authz.Request{
				User:             authz.User{UserName: testUsername, Groups: []authz.UserGroup{"Groupxz"}},
				ResourceTypeName: authz.ResourceTypeKeyConfiguration,
				Action:           authz.ActionKeyConfigurationDelete,
				TenantID:         "tenant3",
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
					_, _ = authHandler.IsAllowed(ctx, req.request)
				}
			},
		)
	}
}

// RoleAssignment handles the logic for assigning roles based on the entity index
type RoleAssignment struct {
	roles []constants.Role
}

func newRoleAssignment() *RoleAssignment {
	return &RoleAssignment{
		roles: []constants.Role{
			constants.TenantAuditorRole,
			constants.TenantAdminRole,
			constants.KeyAdminRole,
		},
	}
}

func (ra *RoleAssignment) getRoleForIndex(idx int) constants.Role {
	if len(ra.roles) == 0 || idx < 0 {
		return ""
	}

	return ra.roles[idx%len(ra.roles)]
}

func createTestEntities(totalCount int) []authz.Entity {
	if totalCount%entitiesPerTenant != 0 {
		return nil
	}

	numTenants := totalCount / entitiesPerTenant
	entities := make([]authz.Entity, 0, numTenants*entitiesPerTenant)

	userGroups := generateUserGroups()
	roleAssigner := newRoleAssignment()

	for tenantIdx := range numTenants {
		entities = append(entities, createEntitiesForTenant(tenantIdx, userGroups, roleAssigner)...)
	}

	return entities
}

func generateUserGroups() []authz.UserGroup {
	groups := make([]authz.UserGroup, maxGroupCount)

	for i := range maxGroupCount {
		groups[i] = authz.UserGroup(fmt.Sprintf("Group%d", i+1))
	}

	return groups
}

func createEntitiesForTenant(tenantIdx int, allGroups []authz.UserGroup, roleAssigner *RoleAssignment) []authz.Entity {
	entities := make([]authz.Entity, entitiesPerTenant)
	tenantID := authz.TenantID(fmt.Sprintf("tenant%d", tenantIdx+1))

	for entityIdx := range entitiesPerTenant {
		globalIdx := tenantIdx*entitiesPerTenant + entityIdx
		groupCount := (globalIdx % maxGroupCount) + 1

		// Ensure groupCount does not exceed the length of allGroups
		safeGroupCount := groupCount
		if safeGroupCount > len(allGroups) {
			safeGroupCount = len(allGroups)
		}

		entities[entityIdx] = authz.Entity{
			TenantID:   tenantID,
			Role:       roleAssigner.getRoleForIndex(globalIdx),
			UserGroups: allGroups[:safeGroupCount],
		}
	}

	return entities
}
