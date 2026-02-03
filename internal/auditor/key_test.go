package auditor_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/constants"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func getCmkLifeCycleAuditTestCases(eventType string) []struct {
	name       string
	cmkID      string
	tenantID   string
	expErr     error
	statusCode int
} {
	return []struct {
		name       string
		cmkID      string
		tenantID   string
		expErr     error
		statusCode int
	}{
		{
			name:       "valid " + eventType + " audit log",
			cmkID:      generateTestCmkID(),
			tenantID:   uuid.NewString(),
			statusCode: http.StatusOK,
		},
		{
			name:       "missing tenant ID in context",
			cmkID:      generateTestCmkID(),
			tenantID:   "",
			statusCode: http.StatusOK,
			expErr:     auditor.ErrCreateEventMetadata,
		},
		{
			name:       "empty cmkID",
			cmkID:      "",
			tenantID:   uuid.NewString(),
			statusCode: http.StatusOK,
			expErr:     auditor.ErrCreateEvent,
		},
		{
			name:       "collector server error",
			cmkID:      generateTestCmkID(),
			tenantID:   uuid.NewString(),
			statusCode: http.StatusInternalServerError,
			expErr:     auditor.ErrSendEvent,
		},
	}
}

// Generic test function for all CMK keys audit methods
func testCmkAuditMethod(
	t *testing.T, eventType string,
	auditMethod func(*auditor.Auditor, context.Context, string) error,
) {
	t.Helper()

	tests := getCmkLifeCycleAuditTestCases(eventType)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(r.Body)
					assert.NoError(t, err)

					unmarshaler := plog.JSONUnmarshaler{}
					logs, err := unmarshaler.UnmarshalLogs(body)
					assert.NoError(t, err)

					eventName, err := getEventName(&logs)
					assert.NoError(t, err)
					assert.Equal(t, tt.cmkID, eventName)

					attrs, err := getAttributes(&logs)
					assert.NoError(t, err)
					assert.Equal(t, eventType, attrs[otlpaudit.EventTypeKey])
					assert.Equal(t, tt.tenantID, attrs[otlpaudit.TenantIDKey])
					assert.Equal(t, tt.cmkID, attrs[otlpaudit.ObjectIDKey])
					assert.Equal(t, constants.SystemUser.String(), attrs[otlpaudit.UserInitiatorIDKey])

					w.WriteHeader(tt.statusCode)
				}))
			defer server.Close()

			testAuditor := createTestAuditor(server.URL)

			ctx := cmkcontext.CreateTenantContext(t.Context(), tt.tenantID)
			err := auditMethod(testAuditor, ctx, tt.cmkID)

			if tt.expErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expErr)

				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestAuditor_SendCmkCreateAuditLog(t *testing.T) {
	testCmkAuditMethod(t, otlpaudit.CmkCreateEvent, func(a *auditor.Auditor, ctx context.Context, cmkID string) error {
		return a.SendCmkCreateAuditLog(ctx, cmkID)
	})
}

func TestAuditor_SendCmkDeleteAuditLog(t *testing.T) {
	testCmkAuditMethod(t, otlpaudit.CmkDeleteEvent, func(a *auditor.Auditor, ctx context.Context, cmkID string) error {
		return a.SendCmkDeleteAuditLog(ctx, cmkID)
	})
}

func TestAuditor_SendCmkDetachAuditLog(t *testing.T) {
	testCmkAuditMethod(t, otlpaudit.CmkDetachEvent, func(a *auditor.Auditor, ctx context.Context, cmkID string) error {
		return a.SendCmkDetachAuditLog(ctx, cmkID)
	})
}

func TestAuditor_SendCmkEnableAuditLog(t *testing.T) {
	testCmkAuditMethod(t, otlpaudit.CmkEnableEvent, func(a *auditor.Auditor, ctx context.Context, cmkID string) error {
		return a.SendCmkEnableAuditLog(ctx, cmkID)
	})
}

func TestAuditor_SendCmkDisableAuditLog(t *testing.T) {
	testCmkAuditMethod(t, otlpaudit.CmkDisableEvent, func(a *auditor.Auditor, ctx context.Context, cmkID string) error {
		return a.SendCmkDisableAuditLog(ctx, cmkID)
	})
}

func TestAuditor_SendCmkRotateAuditLog(t *testing.T) {
	testCmkAuditMethod(t, otlpaudit.CmkRotateEvent, func(a *auditor.Auditor, ctx context.Context, cmkID string) error {
		return a.SendCmkRotateAuditLog(ctx, cmkID)
	})
}

func TestAuditor_SendCmkAvailableAuditLog(t *testing.T) {
	testCmkAuditMethod(t,
		otlpaudit.CmkAvailableEvent,
		func(a *auditor.Auditor, ctx context.Context, cmkID string) error {
			return a.SendCmkAvailableAuditLog(ctx, cmkID)
		})
}

func TestAuditor_SendCmkUnavailableAuditLog(t *testing.T) {
	testCmkAuditMethod(t,
		otlpaudit.CmkUnavailableEvent,
		func(a *auditor.Auditor, ctx context.Context, cmkID string) error {
			return a.SendCmkUnavailableAuditLog(ctx, cmkID)
		})
}
