package dpp_test

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// reEmail matches any email address in a response body.
var reEmail = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

// TestNoPIIInErrorResponses verifies that API error responses do not echo
// personal data fields (user identity, group names, auth context values) back
// to the caller. Error responses must contain only structured error codes and
// generic messages, not internal identity details.
//
// Note: successful (2xx) responses are permitted to contain identity data —
// for example, group listings and workflow approver selection legitimately
// expose user identifiers to authenticated users within the same tenant.
// This test targets error paths only, where identity leakage is never
// intentional.
func TestNoPIIInErrorResponses(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	tenant := tenants[0]
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

	r := sql.NewRepository(db)
	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	sv := testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{})

	// piiValues are real-looking personal data values that must not be
	// reflected back in any error response body.
	piiValues := []string{
		authClient.Identifier,
		authClient.Group.IAMIdentifier,
		tenant,
	}

	t.Run("404 for unknown resource does not echo identity", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodGet,
			Endpoint:          "/systems/00000000-0000-0000-0000-000000000000",
			Tenant:            tenant,
			AdditionalContext: authClient.GetClientMap(),
		})
		require.Equal(t, http.StatusNotFound, w.Code)

		body := w.Body.String()
		assertNoPIIInBody(t, body, piiValues)
	})

	t.Run("400 for malformed request does not echo identity", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodGet,
			Endpoint:          "/systems?$filter=invalid filter syntax !!!",
			Tenant:            tenant,
			AdditionalContext: authClient.GetClientMap(),
		})
		require.Equal(t, http.StatusBadRequest, w.Code)

		body := w.Body.String()
		assertNoPIIInBody(t, body, piiValues)
	})
}

func assertNoPIIInBody(t *testing.T, body string, piiValues []string) {
	t.Helper()

	// Ensure the response is valid JSON — error responses must be structured.
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &parsed),
		"error response must be valid JSON, got: %s", body)

	for _, pii := range piiValues {
		if pii == "" {
			continue
		}
		assert.NotContains(t, body, pii,
			"error response must not contain personal data value %q", pii)
	}

	assert.Empty(t, reEmail.FindString(body),
		"error response must not contain an email address, got: %s", body)
}
