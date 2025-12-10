package auditor_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"

	"github.tools.sap/kms/cmk/internal/auditor"
	"github.tools.sap/kms/cmk/internal/config"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

func createTestContext() context.Context {
	ctx := cmkcontext.InjectRequestID(context.Background())
	ctx = cmkcontext.CreateTenantContext(ctx, "test-tenant-123")

	return ctx
}

func TestNewAuditor(t *testing.T) {
	cfg := &config.Config{}
	ctx := t.Context()

	aud := auditor.New(ctx, cfg)

	assert.NotNil(t, aud)
}

func TestGetEventMetadata(t *testing.T) {
	cfg := &config.Config{}

	tests := []struct {
		name      string
		auditor   *auditor.Auditor
		setupCtx  func() context.Context
		wantError bool
		errorMsg  string
	}{
		{
			name:      "nil auditor",
			auditor:   nil,
			setupCtx:  context.Background,
			wantError: true,
			errorMsg:  auditor.ErrNilAuditor.Error(),
		},
		{
			name:      "missing tenant id",
			auditor:   auditor.New(context.Background(), cfg),
			setupCtx:  context.Background,
			wantError: true,
			errorMsg:  "failed to create event metadata",
		},
		{
			name:      "success with tenant id",
			auditor:   auditor.New(context.Background(), cfg),
			setupCtx:  createTestContext,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				metadata any
				err      error
				ctx      = tt.setupCtx()
			)

			if tt.auditor != nil {
				metadata, err = tt.auditor.GetEventMetadata(ctx)
			} else {
				metadata, err = (*auditor.Auditor)(nil).GetEventMetadata(ctx)
			}

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, metadata)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, metadata)
			}
		})
	}
}

func TestSendEvent(t *testing.T) {
	cfg := &config.Config{}

	successCreateFn := func(_ otlpaudit.EventMetadata) (plog.Logs, error) {
		return plog.NewLogs(), nil
	}

	failCreateFn := func(_ otlpaudit.EventMetadata) (plog.Logs, error) {
		return plog.Logs{}, auditor.ErrCreateEvent
	}

	tests := []struct {
		name          string
		auditor       *auditor.Auditor
		setupCtx      func() context.Context
		createEventFn func(otlpaudit.EventMetadata) (plog.Logs, error)
		wantError     bool
		errorMsg      string
	}{
		{
			name:          "nil auditor",
			auditor:       nil,
			setupCtx:      context.Background,
			createEventFn: successCreateFn,
			wantError:     true,
			errorMsg:      auditor.ErrNilAuditor.Error(),
		},
		{
			name:          "nil audit logger",
			auditor:       &auditor.Auditor{},
			setupCtx:      context.Background,
			createEventFn: successCreateFn,
			wantError:     false,
		},
		{
			name:          "missing tenant id",
			auditor:       auditor.New(context.Background(), cfg),
			setupCtx:      context.Background,
			createEventFn: successCreateFn,
			wantError:     true,
			errorMsg:      "failed to create event metadata",
		},
		{
			name:          "event creation failure",
			auditor:       auditor.New(context.Background(), cfg),
			setupCtx:      createTestContext,
			createEventFn: failCreateFn,
			wantError:     true,
			errorMsg:      "failed to create audit event",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error

			ctx := tt.setupCtx()

			if tt.auditor != nil {
				err = tt.auditor.SendEvent(ctx, tt.createEventFn)
			} else {
				err = (*auditor.Auditor)(nil).SendEvent(ctx, tt.createEventFn)
			}

			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
