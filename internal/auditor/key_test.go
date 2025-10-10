package auditor_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/config"
)

func createTestAuditor(endpoint string) *auditor.Auditor {
	cfg := config.Config{
		BaseConfig: commoncfg.BaseConfig{Audit: commoncfg.Audit{Endpoint: endpoint}},
	}

	return auditor.New(context.Background(), &cfg)
}

func generateTestCmkID() string {
	return uuid.New().String()
}

func getCmkLifeCycleAuditTestCases(methodName string) []struct {
	name                    string
	setupCtx                func() context.Context
	cmkID                   string
	expectErr               bool
	errType                 error
	mockCollectorStatusCode int
} {
	return []struct {
		name      string
		setupCtx  func() context.Context
		cmkID     string
		expectErr bool
		errType   error

		mockCollectorStatusCode int
	}{
		{
			name:                    "valid " + methodName + " audit log",
			setupCtx:                createTestContext,
			cmkID:                   generateTestCmkID(),
			expectErr:               false,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "invalid context",
			setupCtx:                context.Background,
			cmkID:                   generateTestCmkID(),
			expectErr:               true,
			errType:                 auditor.ErrCreateEventMetadata,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "empty cmkID",
			setupCtx:                createTestContext,
			cmkID:                   "",
			expectErr:               true,
			errType:                 auditor.ErrCreateEvent,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "collector server error",
			setupCtx:                createTestContext,
			cmkID:                   generateTestCmkID(),
			expectErr:               true,
			errType:                 auditor.ErrSendEvent,
			mockCollectorStatusCode: http.StatusInternalServerError,
		},
	}
}

// Generic test function for all CMK keys audit methods
func testCmkAuditMethod(
	t *testing.T, methodName string,
	auditMethod func(*auditor.Auditor, context.Context, string) error,
) {
	t.Helper()

	tests := getCmkLifeCycleAuditTestCases(methodName)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCollectorServer := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tt.mockCollectorStatusCode)
				}))
			defer mockCollectorServer.Close()

			testAuditor := createTestAuditor(mockCollectorServer.URL)
			err := auditMethod(testAuditor, tt.setupCtx(), tt.cmkID)

			if tt.expectErr {
				assert.Error(t, err)

				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAuditor_SendCmkCreateAuditLog(t *testing.T) {
	testCmkAuditMethod(t, "cmk create", func(a *auditor.Auditor, ctx context.Context, cmkID string) error {
		return a.SendCmkCreateAuditLog(ctx, cmkID)
	})
}

func TestAuditor_SendCmkDeleteAuditLog(t *testing.T) {
	testCmkAuditMethod(t, "cmk delete", func(a *auditor.Auditor, ctx context.Context, cmkID string) error {
		return a.SendCmkDeleteAuditLog(ctx, cmkID)
	})
}

func TestAuditor_SendCmkEnableAuditLog(t *testing.T) {
	testCmkAuditMethod(t, "cmk enable", func(a *auditor.Auditor, ctx context.Context, cmkID string) error {
		return a.SendCmkEnableAuditLog(ctx, cmkID)
	})
}

func TestAuditor_SendCmkDisableAuditLog(t *testing.T) {
	testCmkAuditMethod(t, "cmk disable", func(a *auditor.Auditor, ctx context.Context, cmkID string) error {
		return a.SendCmkDisableAuditLog(ctx, cmkID)
	})
}

func TestAuditor_SendCmkRotateAuditLog(t *testing.T) {
	testCmkAuditMethod(t, "cmk rotate", func(a *auditor.Auditor, ctx context.Context, cmkID string) error {
		return a.SendCmkRotateAuditLog(ctx, cmkID)
	})
}
