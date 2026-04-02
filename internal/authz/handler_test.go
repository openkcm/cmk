package authz_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/stretchr/testify/assert"
)

// TestIsAllowed tests the IsAllowed function of the AuthorizationHandler
func TestIsAllowed(t *testing.T) {
	tests := []struct {
		name               string
		entities           []authz.Entity[constants.BusinessRole, authz.BusinessUserEntity]
		request            authz.Request[authz.BusinessUserRequest, authz.APIResourceTypeName, authz.APIAction]
		expectError        bool
		expectedErrHandler bool
		expectAllow        bool
		tenantID           authz.TenantID
	}{
		{
			name: "NoExistentRole",
			entities: []authz.Entity[constants.BusinessRole, authz.BusinessUserEntity]{
				{
					User: authz.BusinessUserEntity{
						TenantID: "tenant1",
						Groups:   []string{"Group1"},
					},
					Role: "NonExistentRole",
				},
			},
			request: authz.Request[authz.BusinessUserRequest, authz.APIResourceTypeName, authz.APIAction]{
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
			entities: []authz.Entity[constants.BusinessRole, authz.BusinessUserEntity]{},
			request: authz.Request[authz.BusinessUserRequest, authz.APIResourceTypeName, authz.APIAction]{
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
			entities: []authz.Entity[constants.BusinessRole, authz.BusinessUserEntity]{
				{
					User: authz.BusinessUserEntity{
						TenantID: "tenant1",
						Groups:   []string{"Group1"},
					},
					Role: constants.TenantAdminRole,
				},
			},
			request: authz.Request[authz.BusinessUserRequest, authz.APIResourceTypeName, authz.APIAction]{
				User: authz.BusinessUserRequest{
					TenantID: "tenant1",
					UserName: "test_user",
					Groups:   []string{"Group1"},
				},
				ResourceTypeName: authz.APIResourceTypeName(""),
				Action:           authz.APIAction(""),
			},
			expectError:        true,
			expectedErrHandler: false,
			expectAllow:        false,
			tenantID:           "tenant1",
		},
		{
			name: "EmptyTenant",
			entities: []authz.Entity[constants.BusinessRole, authz.BusinessUserEntity]{
				{
					User: authz.BusinessUserEntity{
						TenantID: "tenant1",
						Groups:   []string{"Group1"},
					},
					Role: constants.KeyAdminRole,
				},
			},
			request: authz.Request[authz.BusinessUserRequest, authz.APIResourceTypeName, authz.APIAction]{
				User: authz.BusinessUserRequest{
					TenantID: authz.EmptyTenantID,
					UserName: "test_user",
					Groups:   []string{"Group1"},
				},
				ResourceTypeName: authz.APIResourceTypeKey,
				Action:           authz.APIActionRead,
			},
			expectError:        true,
			expectedErrHandler: false,
			expectAllow:        false,
			tenantID:           authz.EmptyTenantID,
		},
		{
			name: "ValidRequestWithAllowedAction",
			entities: []authz.Entity[constants.BusinessRole, authz.BusinessUserEntity]{
				{
					User: authz.BusinessUserEntity{
						TenantID: "tenant1",
						Groups:   []string{"Group1"},
					},
					Role: constants.KeyAdminRole,
				},
			},
			request: authz.Request[authz.BusinessUserRequest, authz.APIResourceTypeName, authz.APIAction]{
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
			entities: []authz.Entity[constants.BusinessRole, authz.BusinessUserEntity]{
				{
					User: authz.BusinessUserEntity{
						TenantID: "tenant1",
						Groups:   []string{"Group1"},
					},
					Role: constants.KeyAdminRole,
				},
			},
			request: authz.Request[authz.BusinessUserRequest, authz.APIResourceTypeName, authz.APIAction]{
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
				APIInternalPolicies := make(
					map[constants.InternalRole][]authz.BasePolicy[constants.InternalRole,
						authz.APIResourceTypeName, authz.APIAction])
				authHandler, err := authz.NewAuthorizationHandler(audit, APIInternalPolicies,
					authz.APIBusinessPolicies, authz.APIResourceTypeActions)
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
				ctx = context.WithValue(ctx, constants.Source, constants.BusinessSource)

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
	APIInternalPolicies := make(map[constants.InternalRole][]authz.BasePolicy[constants.InternalRole,
		authz.APIResourceTypeName, authz.APIAction])
	authHandler, err := authz.NewAuthorizationHandler(audit,
		APIInternalPolicies, authz.APIBusinessPolicies, authz.APIResourceTypeActions)
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
			authz.APIResourceTypeName, authz.APIAction]
		tenantID authz.TenantID
	}{
		{
			name: "singleGroupCommonAccess",
			request: authz.Request[authz.BusinessUserRequest,
				authz.APIResourceTypeName, authz.APIAction]{
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
				authz.APIResourceTypeName, authz.APIAction]{
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
				authz.APIResourceTypeName, authz.APIAction]{
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

func createTestEntities(totalCount int) []authz.Entity[constants.BusinessRole, authz.BusinessUserEntity] {
	if totalCount%entitiesPerTenant != 0 {
		return nil
	}

	numTenants := totalCount / entitiesPerTenant
	entities := make([]authz.Entity[constants.BusinessRole, authz.BusinessUserEntity], 0, numTenants*entitiesPerTenant)

	userGroups := generateUserGroups()
	roleAssigner := newRoleAssignment()

	for tenantIdx := range numTenants {
		entities = append(entities, createEntitiesForTenant(tenantIdx, userGroups, roleAssigner)...)
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
	tenantIdx int, allGroups []string, roleAssigner *RoleAssignment,
) []authz.Entity[constants.BusinessRole, authz.BusinessUserEntity] {
	entities := make([]authz.Entity[constants.BusinessRole, authz.BusinessUserEntity], entitiesPerTenant)
	tenantID := authz.TenantID(fmt.Sprintf("tenant%d", tenantIdx+1))

	for entityIdx := range entitiesPerTenant {
		globalIdx := tenantIdx*entitiesPerTenant + entityIdx
		groupCount := (globalIdx % maxGroupCount) + 1

		// Ensure groupCount does not exceed the length of allGroups
		safeGroupCount := min(groupCount, len(allGroups))

		entities[entityIdx] = authz.Entity[constants.BusinessRole, authz.BusinessUserEntity]{
			User: authz.BusinessUserEntity{
				TenantID: tenantID,
				Groups:   allGroups[:safeGroupCount],
			},
			Role: roleAssigner.getRoleForIndex(globalIdx),
		}
	}

	return entities
}
