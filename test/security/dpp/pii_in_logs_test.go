package dpp_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// piiFields are the personal data field keys that must not appear in logs at
// the production log level (info/warn). These are fields confirmed present in
// debug-level statements during DPP code review.
var piiFields = []string{
	"group",
	"iamIdentifiers",
	"UserName",
	"userName",
}

// newInfoLevelLogBuffer returns a logger at info level and the buffer it writes to.
// This simulates the production log level.
func newInfoLevelLogBuffer() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	handler := slogctx.NewHandler(slog.NewJSONHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}), nil)
	return slog.New(handler), buf
}

// TestNoPIIInLogsAtProductionLogLevel verifies that personal data fields
// (group memberships, IAM identifiers, user names) are not written to logs
// when the logger is set to info level (the production log level).
// These fields appear only in debug-level statements and must not be visible
// in production logs.
func TestNoPIIInLogsAtProductionLogLevel(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	tenant := tenants[0]
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)

	r := sql.NewRepository(db)
	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())

	logger, buf := newInfoLevelLogBuffer()
	slog.SetDefault(logger)

	sv := testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{})

	// Make an authenticated API call that exercises the business user data
	// middleware — the code path that logs group and auth context fields.
	w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
		Method:            http.MethodGet,
		Endpoint:          "/keyConfigurations",
		Tenant:            tenant,
		AdditionalContext: authClient.GetClientMap(),
	})
	require.Equal(t, http.StatusOK, w.Code)

	logOutput := buf.String()
	for _, field := range piiFields {
		assert.NotContains(t, logOutput, `"`+field+`":`,
			"PII field %q must not appear in logs at production log level (info)", field)
	}
}
