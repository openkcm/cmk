package bounds_test

// TestWorkflowConfigBounds verifies that the PATCH /tenantConfigurations/workflow
// endpoint enforces hard bounds on every configurable field.
//
// Workflow settings control approval counts and key-material retention windows.
// Accepting out-of-range values would either reduce the security posture below
// the agreed minimum (too-low approvals / too-short retention) or create
// operational problems (unbounded growth). Both directions are blocked.
//
// Coverage approach:
//   - One sub-test per field per violated boundary — each failing case is
//     independent so a regression is easy to pinpoint.
//   - A single "all in-range" sanity test confirms valid values are accepted.
//   - Tests are end-to-end: real Postgres container, full HTTP stack, no mocks.

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

// setupBoundsTest spins up a fresh DB + API server and seeds a default workflow
// config so every PATCH test starts from a known valid state.
func setupBoundsTest(t *testing.T) (cmkapi.ServeMux, string, http.Header) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})
	tenant := tenants[0]

	cfg := testutils.TestAPIServerConfig{
		EnableBusinessUserDataMW: true,
	}
	cfg.Config.Database = dbCfg

	keyStorage := testutils.NewTestSigningKeyStorage(t)
	cfg.SigningKeyStorage = keyStorage

	sv := testutils.NewAPIServer(t, db, cfg)
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithTenantAdminRole())

	// Seed a valid baseline workflow config so PATCH merges against known state.
	testutils.WriteWorkflowConfig(ctx, t, r, testutils.NewDefaultWorkflowConfig(true))

	businessUserData := &auth.ClientData{
		Identifier: authClient.Identifier,
		Groups:     []string{authClient.Group.IAMIdentifier},
	}

	privateKey, ok := keyStorage.GetPrivateKey(0)
	require.True(t, ok)
	headers := testutils.NewSignedBusinessUserDataHeaders(t, businessUserData, privateKey, 0)

	return sv, tenant, headers
}

// patchWorkflow is a helper that sends a PATCH request and returns the response code.
func patchWorkflow(t *testing.T, sv cmkapi.ServeMux, tenant string, headers http.Header, body cmkapi.TenantWorkflowConfiguration) int {
	t.Helper()

	w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
		Method:   http.MethodPatch,
		Endpoint: "/tenantConfigurations/workflow",
		Tenant:   tenant,
		Body:     testutils.WithJSON(t, body),
		Headers:  headers,
	})
	return w.Code
}

// TestWorkflowConfigOutOfBoundsRejected verifies that every field rejects values
// outside the hard limits defined in constants/workflow.go.
//
// Each sub-test is independent (its own DB container) so a single rogue write
// cannot affect a sibling test.
func TestWorkflowConfigOutOfBoundsRejected(t *testing.T) {
	t.Run("minimumApprovals below minimum (< 2) is rejected", func(t *testing.T) {
		sv, tenant, headers := setupBoundsTest(t)
		code := patchWorkflow(t, sv, tenant, headers, cmkapi.TenantWorkflowConfiguration{
			MinimumApprovals: new(1),
		})
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("minimumApprovals above maximum (> 5) is rejected", func(t *testing.T) {
		sv, tenant, headers := setupBoundsTest(t)
		code := patchWorkflow(t, sv, tenant, headers, cmkapi.TenantWorkflowConfiguration{
			MinimumApprovals: new(6),
		})
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("retentionPeriodDays below minimum (< 7) is rejected", func(t *testing.T) {
		sv, tenant, headers := setupBoundsTest(t)
		code := patchWorkflow(t, sv, tenant, headers, cmkapi.TenantWorkflowConfiguration{
			RetentionPeriodDays: new(6),
		})
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("retentionPeriodDays above maximum (> 30) is rejected", func(t *testing.T) {
		sv, tenant, headers := setupBoundsTest(t)
		code := patchWorkflow(t, sv, tenant, headers, cmkapi.TenantWorkflowConfiguration{
			RetentionPeriodDays: new(31),
		})
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("maxExpiryPeriodDays above maximum (> 7) is rejected", func(t *testing.T) {
		sv, tenant, headers := setupBoundsTest(t)
		code := patchWorkflow(t, sv, tenant, headers, cmkapi.TenantWorkflowConfiguration{
			MaxExpiryPeriodDays: new(8),
		})
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("defaultExpiryPeriodDays exceeding maxExpiryPeriodDays is rejected", func(t *testing.T) {
		sv, tenant, headers := setupBoundsTest(t)
		code := patchWorkflow(t, sv, tenant, headers, cmkapi.TenantWorkflowConfiguration{
			DefaultExpiryPeriodDays: new(7),
			MaxExpiryPeriodDays:     new(5),
		})
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("disabling workflow on non-TEST tenant is rejected", func(t *testing.T) {
		sv, tenant, headers := setupBoundsTest(t)
		code := patchWorkflow(t, sv, tenant, headers, cmkapi.TenantWorkflowConfiguration{
			Enabled: new(false),
		})
		assert.Equal(t, http.StatusBadRequest, code)
	})
}

// TestWorkflowConfigInRangeAccepted is a sanity test confirming that a payload
// with all fields at valid in-range values is accepted with 200 OK.
func TestWorkflowConfigInRangeAccepted(t *testing.T) {
	sv, tenant, headers := setupBoundsTest(t)

	w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
		Method:   http.MethodPatch,
		Endpoint: "/tenantConfigurations/workflow",
		Tenant:   tenant,
		Body: testutils.WithJSON(t, cmkapi.TenantWorkflowConfiguration{
			MinimumApprovals:        new(2), // minimum valid
			RetentionPeriodDays:     new(7), // minimum valid
			MaxExpiryPeriodDays:     new(7), // maximum valid
			DefaultExpiryPeriodDays: new(6), // below maxExpiryPeriodDays
		}),
		Headers: headers,
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp cmkapi.TenantWorkflowConfiguration
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	require.NotNil(t, resp.MinimumApprovals)
	assert.Equal(t, 2, *resp.MinimumApprovals)

	require.NotNil(t, resp.RetentionPeriodDays)
	assert.Equal(t, 7, *resp.RetentionPeriodDays)

	require.NotNil(t, resp.MaxExpiryPeriodDays)
	assert.Equal(t, 7, *resp.MaxExpiryPeriodDays)

	require.NotNil(t, resp.DefaultExpiryPeriodDays)
	assert.Equal(t, 6, *resp.DefaultExpiryPeriodDays)
}
