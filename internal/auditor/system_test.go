package auditor_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"

	"github.com/openkcm/cmk/internal/auditor"
)

func testCmkSystemAuditMethod(
	t *testing.T,
	methodName string,
	auditMethod func(*auditor.Auditor, context.Context, string, string) error,
) {
	t.Helper()

	tests := []struct {
		name                    string
		setupCtx                func() context.Context
		cmkID                   string
		systemID                string
		expectErr               bool
		errType                 error
		mockCollectorStatusCode int
	}{
		{
			name:                    "valid " + methodName + " audit log",
			setupCtx:                createTestContext,
			cmkID:                   generateTestCmkID(),
			systemID:                "test-system-123",
			expectErr:               false,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "invalid context",
			setupCtx:                context.Background,
			cmkID:                   generateTestCmkID(),
			systemID:                "test-system-123",
			expectErr:               true,
			errType:                 auditor.ErrCreateEventMetadata,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "empty cmkID",
			setupCtx:                createTestContext,
			cmkID:                   "",
			systemID:                "test-system-123",
			expectErr:               true,
			errType:                 auditor.ErrCreateEvent,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "empty systemID",
			setupCtx:                createTestContext,
			cmkID:                   generateTestCmkID(),
			systemID:                "",
			expectErr:               true,
			errType:                 auditor.ErrCreateEvent,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "collector server error",
			setupCtx:                createTestContext,
			cmkID:                   generateTestCmkID(),
			systemID:                "test-system-123",
			expectErr:               true,
			errType:                 auditor.ErrSendEvent,
			mockCollectorStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCollectorServer := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tt.mockCollectorStatusCode)
				}))
			defer mockCollectorServer.Close()

			testAuditor := createTestAuditor(mockCollectorServer.URL)

			err := auditMethod(testAuditor, tt.setupCtx(), tt.systemID, tt.cmkID)

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

func testCmkSwitchAuditMethod(
	t *testing.T,
	methodName string,
	auditMethod func(*auditor.Auditor, context.Context, string, string, string) error,
) {
	t.Helper()

	tests := []struct {
		name                    string
		setupCtx                func() context.Context
		systemID                string
		cmkIDOld                string
		cmkIDNew                string
		expectErr               bool
		errType                 error
		mockCollectorStatusCode int
	}{
		{
			name:                    "valid " + methodName + " audit log",
			setupCtx:                createTestContext,
			systemID:                "test-system-123",
			cmkIDOld:                generateTestCmkID(),
			cmkIDNew:                generateTestCmkID(),
			expectErr:               false,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "invalid context",
			setupCtx:                context.Background,
			systemID:                "test-system-123",
			cmkIDOld:                generateTestCmkID(),
			cmkIDNew:                generateTestCmkID(),
			expectErr:               true,
			errType:                 auditor.ErrCreateEventMetadata,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "empty systemID",
			setupCtx:                createTestContext,
			systemID:                "",
			cmkIDOld:                generateTestCmkID(),
			cmkIDNew:                generateTestCmkID(),
			expectErr:               true,
			errType:                 auditor.ErrCreateEvent,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "empty cmkIDOld",
			setupCtx:                createTestContext,
			systemID:                "test-system-123",
			cmkIDOld:                "",
			cmkIDNew:                generateTestCmkID(),
			expectErr:               true,
			errType:                 auditor.ErrCreateEvent,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "empty cmkIDNew",
			setupCtx:                createTestContext,
			systemID:                "test-system-123",
			cmkIDOld:                generateTestCmkID(),
			cmkIDNew:                "",
			expectErr:               true,
			errType:                 auditor.ErrCreateEvent,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "collector server error",
			setupCtx:                createTestContext,
			systemID:                "test-system-123",
			cmkIDOld:                generateTestCmkID(),
			cmkIDNew:                generateTestCmkID(),
			expectErr:               true,
			errType:                 auditor.ErrSendEvent,
			mockCollectorStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCollectorServer := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tt.mockCollectorStatusCode)
				}))
			defer mockCollectorServer.Close()

			testAuditor := createTestAuditor(mockCollectorServer.URL)

			err := auditMethod(testAuditor, tt.setupCtx(), tt.systemID, tt.cmkIDOld, tt.cmkIDNew)

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

func testCmkTenantModificationAuditMethod(
	t *testing.T,
	methodName string,
	auditMethod func(*auditor.Auditor, context.Context, string, string, otlpaudit.CmkAction) error,
) {
	t.Helper()

	tests := []struct {
		name                    string
		setupCtx                func() context.Context
		cmkID                   string
		systemID                string
		action                  otlpaudit.CmkAction
		expectErr               bool
		errType                 error
		mockCollectorStatusCode int
	}{
		{
			name:                    "valid " + methodName + " audit log",
			setupCtx:                createTestContext,
			cmkID:                   generateTestCmkID(),
			systemID:                "test-system-123",
			action:                  otlpaudit.CMKACTION_ONBOARD,
			expectErr:               false,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "invalid context",
			setupCtx:                context.Background,
			cmkID:                   generateTestCmkID(),
			systemID:                "test-system-123",
			action:                  otlpaudit.CMKACTION_ONBOARD,
			expectErr:               true,
			errType:                 auditor.ErrCreateEventMetadata,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "empty cmkID",
			setupCtx:                createTestContext,
			cmkID:                   "",
			systemID:                "test-system-123",
			action:                  otlpaudit.CMKACTION_ONBOARD,
			expectErr:               true,
			errType:                 auditor.ErrCreateEvent,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "empty systemID",
			setupCtx:                createTestContext,
			cmkID:                   generateTestCmkID(),
			systemID:                "",
			action:                  otlpaudit.CMKACTION_ONBOARD,
			expectErr:               true,
			errType:                 auditor.ErrCreateEvent,
			mockCollectorStatusCode: http.StatusOK,
		},
		{
			name:                    "collector server error",
			setupCtx:                createTestContext,
			cmkID:                   generateTestCmkID(),
			systemID:                "test-system-123",
			action:                  otlpaudit.CMKACTION_ONBOARD,
			expectErr:               true,
			errType:                 auditor.ErrSendEvent,
			mockCollectorStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCollectorServer := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tt.mockCollectorStatusCode)
				}))
			defer mockCollectorServer.Close()

			testAuditor := createTestAuditor(mockCollectorServer.URL)

			err := auditMethod(testAuditor, tt.setupCtx(), tt.cmkID, tt.systemID, tt.action)

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

func TestAuditor_SendCmkOnboardingAuditLog(t *testing.T) {
	testCmkSystemAuditMethod(
		t,
		"cmk onboarding",
		func(a *auditor.Auditor, ctx context.Context, systemID, cmkID string) error {
			return a.SendCmkOnboardingAuditLog(ctx, systemID, cmkID)
		},
	)
}

func TestAuditor_SendCmkOffboardingAuditLog(t *testing.T) {
	testCmkSystemAuditMethod(
		t,
		"cmk offboarding",
		func(a *auditor.Auditor, ctx context.Context, systemID, cmkID string) error {
			return a.SendCmkOffboardingAuditLog(ctx, systemID, cmkID)
		},
	)
}

func TestAuditor_SendCmkSwitchAuditLog(t *testing.T) {
	testCmkSwitchAuditMethod(
		t,
		"cmk switch",
		func(a *auditor.Auditor, ctx context.Context, systemID, cmkIDOld, cmkIDNew string) error {
			return a.SendCmkSwitchAuditLog(ctx, systemID, cmkIDOld, cmkIDNew)
		},
	)
}

func TestAuditor_SendCmkTenantModificationAuditLog(t *testing.T) {
	testCmkTenantModificationAuditMethod(
		t,
		"cmk tenant modification",
		func(a *auditor.Auditor, ctx context.Context, systemID, cmkID string, action otlpaudit.CmkAction) error {
			return a.SendCmkTenantModificationAuditLog(ctx, systemID, cmkID, action)
		},
	)
}
