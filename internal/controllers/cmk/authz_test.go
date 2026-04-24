//go:build !unit

package cmk_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/daemon"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func startAPIAuthz(t *testing.T) (*multitenancy.DB, *daemon.ServeMux, string) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	})

	sv := testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{
		Config: config.Config{Database: dbCfg},
	})

	return db, sv, tenants[0]
}

// authzEndpoints returns all restricted API endpoints for authz testing.
// Each entry maps to a restriction in authz.RestrictionsByAPI.
func authzEndpoints() []testutils.AuthzTestEndpoint {
	keyID := uuid.New().String()
	keyConfigID := uuid.New().String()
	systemID := uuid.New().String()
	workflowID := uuid.New().String()
	groupID := uuid.New().String()

	return []testutils.AuthzTestEndpoint{
		// --- Keys ---
		{
			Method:   http.MethodGet,
			Endpoint: "/keys?keyConfigurationID=" + keyConfigID,
		},
		{
			Method:   http.MethodPost,
			Endpoint: "/keys",
			Body: `{
				"name": "test-key",
				"keyConfigurationID": "` + keyConfigID + `"
			}`,
		},
		{
			Method:   http.MethodGet,
			Endpoint: "/keys/" + keyID,
		},
		{
			Method:   http.MethodPatch,
			Endpoint: "/keys/" + keyID,
			Body:     `{"name": "updated"}`,
		},
		{
			Method:   http.MethodDelete,
			Endpoint: "/keys/" + keyID,
		},
		{
			Method:   http.MethodGet,
			Endpoint: "/keys/" + keyID + "/importParams",
		},
		{
			Method:   http.MethodPost,
			Endpoint: "/keys/" + keyID + "/importKeyMaterial",
			Body:     `{"encryptedKeyMaterial": "dGVzdA==", "importToken": "dGVzdA=="}`,
		},
		{
			Method:   http.MethodGet,
			Endpoint: "/keys/" + keyID + "/versions",
		},
		// NOTE: POST /keys/{keyID}/versions and GET /keys/{keyID}/versions/{version}
		// are defined in the authz mapping but not registered as API routes.

		// --- Key Labels ---
		{
			Method:   http.MethodGet,
			Endpoint: "/key/" + keyID + "/labels",
		},
		{
			Method:   http.MethodPost,
			Endpoint: "/key/" + keyID + "/labels",
			Body:     `{"labels": {"env": "test"}}`,
		},
		{
			Method:   http.MethodDelete,
			Endpoint: "/key/" + keyID + "/label/testlabel",
		},

		// --- Key Configurations ---
		{
			Method:   http.MethodGet,
			Endpoint: "/keyConfigurations",
		},
		{
			Method:   http.MethodPost,
			Endpoint: "/keyConfigurations",
			Body: `{
				"name": "test-kc",
				"keyAlgorithm": "AES",
				"provider": "TEST"
			}`,
		},
		{
			Method:   http.MethodGet,
			Endpoint: "/keyConfigurations/" + keyConfigID,
		},
		{
			Method:   http.MethodPatch,
			Endpoint: "/keyConfigurations/" + keyConfigID,
			Body:     `{"name": "updated"}`,
		},
		{
			Method:   http.MethodDelete,
			Endpoint: "/keyConfigurations/" + keyConfigID,
		},
		{
			Method:   http.MethodGet,
			Endpoint: "/keyConfigurations/" + keyConfigID + "/tags",
		},
		{
			Method:   http.MethodPut,
			Endpoint: "/keyConfigurations/" + keyConfigID + "/tags",
			Body:     `{"tags": {"env": "test"}}`,
		},
		{
			Method:   http.MethodGet,
			Endpoint: "/keyConfigurations/" + keyConfigID + "/certificates",
		},

		// --- Systems ---
		{
			Method:   http.MethodGet,
			Endpoint: "/systems",
		},
		{
			Method:   http.MethodGet,
			Endpoint: "/systems/" + systemID,
		},
		{
			Method:   http.MethodPatch,
			Endpoint: "/systems/" + systemID + "/link",
			Body:     `{"keyConfigurationID": "` + keyConfigID + `"}`,
		},
		{
			Method:   http.MethodDelete,
			Endpoint: "/systems/" + systemID + "/link",
		},
		{
			Method:   http.MethodPost,
			Endpoint: "/systems/" + systemID + "/recoveryActions",
			Body:     `{"action": "RECOVER"}`,
		},
		{
			Method:   http.MethodGet,
			Endpoint: "/systems/" + systemID + "/recoveryActions",
		},

		// --- Workflows ---
		{
			Method:   http.MethodPost,
			Endpoint: "/workflows",
			Body: `{
				"actionType": "UNLINK",
				"artifactID": "` + systemID + `",
				"artifactType": "SYSTEM"
			}`,
		},
		{
			Method:   http.MethodGet,
			Endpoint: "/workflows",
		},
		{
			Method:   http.MethodPost,
			Endpoint: "/workflows/check",
			Body: `{
				"actionType": "UNLINK",
				"artifactID": "` + systemID + `",
				"artifactType": "SYSTEM"
			}`,
		},
		{
			Method:   http.MethodGet,
			Endpoint: "/workflows/" + workflowID,
		},
		// NOTE: GET/POST /workflows/{workflowID}/approvers are defined in the
		// authz mapping but not registered as API routes.
		{
			Method:   http.MethodPost,
			Endpoint: "/workflows/" + workflowID + "/state",
			Body:     `{"state": "APPROVED"}`,
		},

		// --- Groups ---
		{
			Method:   http.MethodGet,
			Endpoint: "/groups",
		},
		{
			Method:   http.MethodPost,
			Endpoint: "/groups",
			Body: `{
				"name": "test-group",
				"iamIdentifier": "test-iam-id",
				"role": "KEY_ADMINISTRATOR"
			}`,
		},
		{
			Method:   http.MethodPost,
			Endpoint: "/groups/iamCheck",
			Body:     `{"iamIdentifiers": ["test-id"]}`,
		},
		{
			Method:   http.MethodGet,
			Endpoint: "/groups/" + groupID,
		},
		{
			Method:   http.MethodPatch,
			Endpoint: "/groups/" + groupID,
			Body:     `{"name": "updated"}`,
		},
		{
			Method:   http.MethodDelete,
			Endpoint: "/groups/" + groupID,
		},

		// --- Tenant Configurations ---
		{
			Method:   http.MethodGet,
			Endpoint: "/tenantConfigurations/keystores",
		},
		{
			Method:   http.MethodGet,
			Endpoint: "/tenantConfigurations/workflow",
		},
		{
			Method:   http.MethodPatch,
			Endpoint: "/tenantConfigurations/workflow",
			Body:     `{"enabled": true}`,
		},

		// --- Tenant Info ---
		{
			Method:   http.MethodGet,
			Endpoint: "/tenantInfo",
		},
	}
}

// TestAuthzFailures verifies that each restricted endpoint returns 403 Forbidden
// when accessed by a role that does not have the required permission.
// The blocked roles are automatically derived from the authz policy data.
func TestAuthzFailures(t *testing.T) {
	db, sv, tenant := startAPIAuthz(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := sql.NewRepository(db)

	runAuthzFailureTests(t, sv, tenant, r, ctx, authzEndpoints())
}

// runAuthzFailureTests runs authorization failure tests for the provided
// endpoints. For each endpoint it uses the ServeMux to resolve the registered
// pattern, determines which roles should be blocked based on the policy data,
// and asserts that each blocked role receives 403 Forbidden.
func runAuthzFailureTests(
	t *testing.T,
	sv *daemon.ServeMux,
	tenant string,
	r repo.Repo,
	ctx context.Context,
	endpoints []testutils.AuthzTestEndpoint,
) {
	t.Helper()

	for _, ep := range endpoints {
		req := testutils.NewHTTPRequest(t, testutils.RequestOptions{ //nolint:contextcheck
			Method:   ep.Method,
			Endpoint: ep.Endpoint,
			Tenant:   tenant,
		})

		_, pattern := sv.Handler(req)
		pattern = strings.Replace(pattern, sv.BaseURL, "", 1)

		restriction, exists := authz.RestrictionsByAPI[pattern]
		if !exists {
			t.Fatalf(
				"no authz restriction found for pattern %q on %s %s",
				pattern, ep.Method, ep.Endpoint,
			)
		}

		blockedRoles := testutils.GetBlockedRoles(
			restriction.APIResourceTypeName, restriction.APIAction,
		)
		if len(blockedRoles) == 0 {
			t.Logf("all roles are allowed for %q (%s:%s), skipping",
				pattern, restriction.APIResourceTypeName, restriction.APIAction)

			continue
		}

		for _, role := range blockedRoles {
			runAuthzBlockedSubtest(t, sv, tenant, r, ctx, ep, role)
		}
	}
}

// runAuthzBlockedSubtest executes a single subtest asserting that the given
// role is blocked (403 Forbidden) for the endpoint.
func runAuthzBlockedSubtest(
	t *testing.T,
	sv *daemon.ServeMux,
	tenant string,
	r repo.Repo,
	ctx context.Context,
	ep testutils.AuthzTestEndpoint,
	role constants.Role,
) {
	t.Helper()

	testName := fmt.Sprintf(
		"%s_%s_blocked_for_%s", ep.Method, testutils.CleanPath(ep.Endpoint), role,
	)

	t.Run(testName, func(t *testing.T) {
		authClient := testutils.NewAuthClient(ctx, t, r, testutils.RoleAuthClientOpt(role))

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{ //nolint:contextcheck
			Method:            ep.Method,
			Endpoint:          ep.Endpoint,
			Tenant:            tenant,
			Body:              testutils.WithBody(t, ep.Body),
			AdditionalContext: authClient.GetClientMap(),
		})

		assert.Equal(t, http.StatusForbidden, w.Code,
			"expected 403 for role %s on %s %s, got %d: %s",
			role, ep.Method, ep.Endpoint, w.Code, w.Body.String())
	})
}
