package auditor_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"

	"github.tools.sap/kms/cmk/internal/auditor"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/utils/context"
)

func TestSendTenantAuditLog(t *testing.T) {
	tests := []struct {
		name        string
		tenantCtxID string
		tenantID    string
		statusCode  int
		expErr      error
	}{
		{
			name:        "valid " + otlpaudit.CmkTenantDeleteEvent + " audit log",
			tenantCtxID: "tenant",
			tenantID:    "tenant",
			statusCode:  http.StatusOK,
		},
		{
			name:        "missing tenant ID in context",
			tenantCtxID: "",
			tenantID:    "tenant",
			statusCode:  http.StatusOK,
			expErr:      auditor.ErrCreateEventMetadata,
		},
		{
			name:        "missing tenant ID in parameter",
			tenantCtxID: "tenant",
			tenantID:    "",
			statusCode:  http.StatusOK,
			expErr:      auditor.ErrCreateEvent,
		},
		{
			name:        "audit log collector returns error",
			tenantCtxID: "tenant",
			tenantID:    "tenant",
			statusCode:  http.StatusInternalServerError,
			expErr:      auditor.ErrSendEvent,
		},
	}

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
					assert.Equal(t, tt.tenantID, eventName)

					attrs, err := getAttributes(&logs)
					assert.NoError(t, err)
					assert.Equal(t, otlpaudit.CmkTenantDeleteEvent, attrs[otlpaudit.EventTypeKey])
					assert.Equal(t, tt.tenantID, attrs[otlpaudit.TenantIDKey])
					assert.Equal(t, tt.tenantID, attrs[otlpaudit.ObjectIDKey])
					assert.Equal(t, constants.SystemUser.String(), attrs[otlpaudit.UserInitiatorIDKey])

					w.WriteHeader(tt.statusCode)
				}),
			)
			t.Cleanup(server.Close)
			testAuditor := createTestAuditor(server.URL)

			ctx := context.CreateTenantContext(t.Context(), tt.tenantCtxID)
			err := testAuditor.SendCmkTenantDeleteAuditLog(ctx, tt.tenantID)

			if tt.expErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expErr)

				return
			}

			assert.NoError(t, err)
		})
	}
}
