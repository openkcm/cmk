package dpp_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/constants"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// TestAuditLogAttributionNoUUIDMax verifies that internal processes (task-worker,
// event-reconciler, operator etc.) are attributed to their role identifier in
// audit events rather than falling back to uuid.Max
// (00000000-0000-0000-0000-000000000000).
// uuid.Max fallback occurs when neither business user data nor internal role
// is present in the context — internal processes must inject their role.
func TestAuditLogAttributionNoUUIDMax(t *testing.T) {
	tests := []struct {
		name         string
		setupContext func(ctx context.Context) context.Context
		wantUUIDMax  bool
	}{
		{
			name: "internal process with role set — must not use uuid.Max",
			setupContext: func(ctx context.Context) context.Context {
				ctx, err := cmkcontext.InjectInternalUserData(ctx, constants.InternalTaskProcessingRole)
				require.NoError(t, err)
				return ctx
			},
			wantUUIDMax: false,
		},
		{
			name: "internal process with event reconciler role — must not use uuid.Max",
			setupContext: func(ctx context.Context) context.Context {
				ctx, err := cmkcontext.InjectInternalUserData(ctx, constants.InternalEventReconcilerRole)
				require.NoError(t, err)
				return ctx
			},
			wantUUIDMax: false,
		},
		{
			name: "context with no identity — falls back to uuid.Max in auditor",
			setupContext: func(ctx context.Context) context.Context {
				return ctx // no identity injected
			},
			wantUUIDMax: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := cmkcontext.CreateTenantContext(t.Context(), "test-tenant")
			ctx = tt.setupContext(ctx)

			userID, err := cmkcontext.ExtractUserIdentifier(ctx)

			if tt.wantUUIDMax {
				// When no identity is set, ExtractUserIdentifier returns an error
				// and auditor.go falls back to uuid.Max — document this behaviour.
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEqual(t, uuid.Max.String(), userID,
					"internal process audit events must not be attributed to uuid.Max")
				assert.NotEmpty(t, userID,
					"internal process must have a non-empty identity in audit events")
			}
		})
	}
}
