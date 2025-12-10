package auditor_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/auditor"
	"github.tools.sap/kms/cmk/internal/authz"
	"github.tools.sap/kms/cmk/internal/config"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

func createTestContextForAuthz() context.Context {
	ctx := cmkcontext.InjectRequestID(context.Background())
	ctx = cmkcontext.CreateTenantContext(ctx, "test-tenant1")

	return ctx
}

func createTestAuditorForAuthz(endpoint string) *auditor.Auditor {
	cfg := config.Config{
		BaseConfig: commoncfg.BaseConfig{Audit: commoncfg.Audit{Endpoint: endpoint}},
	}

	return auditor.New(context.Background(), &cfg)
}

func TestAuditor_SendCmkUnauthorizedRequestAuditLog(t *testing.T) {
	tests := []struct {
		name                    string
		setupCtx                func() context.Context
		expectErr               bool
		errType                 error
		mockCollectorStatusCode int
	}{
		{
			name:                    "valid unauthorized request audit log",
			setupCtx:                createTestContextForAuthz,
			expectErr:               false,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "invalid context - missing tenant ID",
			setupCtx:                context.Background,
			expectErr:               true,
			errType:                 auditor.ErrCreateEventMetadata,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "collector server error",
			setupCtx:                createTestContextForAuthz,
			expectErr:               true,
			errType:                 auditor.ErrSendEvent,
			mockCollectorStatusCode: http.StatusInternalServerError,
		},
		{
			name:                    "collector server unavailable",
			setupCtx:                createTestContextForAuthz,
			expectErr:               true,
			errType:                 auditor.ErrSendEvent,
			mockCollectorStatusCode: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCollectorServer := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tt.mockCollectorStatusCode)
				}))
			defer mockCollectorServer.Close()

			testAuditor := createTestAuditorForAuthz(mockCollectorServer.URL)
			err := testAuditor.SendCmkUnauthorizedRequestAuditLog(tt.setupCtx(), string(authz.ResourceTypeKeyConfiguration), string(authz.ActionRead))

			if tt.expectErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.errType)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAuditor_SendCmkUnauthorizedRequestAuditLog_NilAuditor(t *testing.T) {
	var testAuditor *auditor.Auditor

	ctx := createTestContextForAuthz()

	err := testAuditor.SendCmkUnauthorizedRequestAuditLog(ctx, string(authz.ResourceTypeKeyConfiguration), string(authz.ActionRead))

	assert.Error(t, err)
	assert.ErrorIs(t, err, auditor.ErrNilAuditor)
}

func TestAuditor_SendCmkUnauthorizedRequestAuditLog_NilAuditLogger(t *testing.T) {
	testAuditor := &auditor.Auditor{}
	ctx := createTestContextForAuthz()

	err := testAuditor.SendCmkUnauthorizedRequestAuditLog(ctx, string(authz.ResourceTypeKeyConfiguration), string(authz.ActionRead))

	// When audit logger is nil, the function should not return an error (it logs a warning and returns nil)
	assert.NoError(t, err)
}
