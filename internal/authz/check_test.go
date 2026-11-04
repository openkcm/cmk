package authz_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// checkAuthzHandler builds a handler with one technical role that can read keys.
func newCheckAuthzHandler(t *testing.T) *authz.Handler[authz.APIResourceType, authz.APIAction] {
	t.Helper()
	cfg := &config.Config{}
	audit := auditor.New(context.Background(), cfg)

	internalPolicies := authz.RolePolicies[constants.InternalRole, authz.APIResourceType, authz.APIAction]{
		constants.InternalRequestingHandlingRole: {
			{
				ID: "InternalPolicy",
				ResourceTypes: []authz.Resource[authz.APIResourceType, authz.APIAction]{
					{Type: authz.APIResourceTypeKey, Actions: []authz.APIAction{authz.APIActionRead}},
				},
			},
		},
	}

	technicalPolicies := authz.RolePolicies[constants.TechnicalRole, authz.APIResourceType, authz.APIAction]{
		"TECH_ROLE": {
			{
				ID: "TechPolicy",
				ResourceTypes: []authz.Resource[authz.APIResourceType, authz.APIAction]{
					{Type: authz.APIResourceTypeKey, Actions: []authz.APIAction{authz.APIActionRead}},
				},
			},
		},
	}

	h, err := authz.NewAuthorizationHandler(
		audit,
		internalPolicies,
		authz.APIPolicies,
		technicalPolicies,
		authz.APIResourceTypeActions,
		&sync.Mutex{},
	)
	assert.NoError(t, err)
	return h
}

func TestCheckAuthz_NoUserType(t *testing.T) {
	h := newCheckAuthzHandler(t)
	ctx := cmkcontext.InjectRequestID(context.Background(), "req-id")

	_, err := authz.CheckAuthz(ctx, h, authz.APIResourceTypeKey, authz.APIActionRead)
	assert.ErrorIs(t, err, authz.ErrAuthzDecision)
}

func TestCheckAuthz_UnknownUserType(t *testing.T) {
	h := newCheckAuthzHandler(t)
	ctx := context.WithValue(context.Background(), constants.UserType, constants.UserTypeValue("Unknown"))
	ctx = cmkcontext.InjectRequestID(ctx, "req-id")

	_, err := authz.CheckAuthz(ctx, h, authz.APIResourceTypeKey, authz.APIActionRead)
	assert.ErrorIs(t, err, authz.ErrAuthzDecision)
	assert.ErrorIs(t, err, authz.ErrNoAuthzForUserType)
}

func TestCheckAuthz_TechnicalUser(t *testing.T) {
	const (
		tenantID = "tenant1"
		techRole = constants.TechnicalRole("TECH_ROLE")
	)

	technicalCtx := func(tid string, role constants.TechnicalRole) context.Context {
		ctx := cmkcontext.CreateTenantContext(context.Background(), tid)
		ctx = cmkcontext.InjectRequestID(ctx, "req-id")
		ctx, _ = cmkcontext.InjectTechnicalUserData(ctx, role)
		return ctx
	}

	tests := []struct {
		name         string
		makeCtx      func() context.Context
		setupHandler func(h *authz.Handler[authz.APIResourceType, authz.APIAction])
		resource     authz.APIResourceType
		action       authz.APIAction
		expectAllow  bool
		expectErr    bool
	}{
		{
			name:    "allowed",
			makeCtx: func() context.Context { return technicalCtx(tenantID, techRole) },
			setupHandler: func(h *authz.Handler[authz.APIResourceType, authz.APIAction]) {
				h.UpdateTechnicalUserData(authz.TenantID(tenantID))
			},
			resource:    authz.APIResourceTypeKey,
			action:      authz.APIActionRead,
			expectAllow: true,
		},
		{
			name:    "denied - action not permitted",
			makeCtx: func() context.Context { return technicalCtx(tenantID, techRole) },
			setupHandler: func(h *authz.Handler[authz.APIResourceType, authz.APIAction]) {
				h.UpdateTechnicalUserData(authz.TenantID(tenantID))
			},
			resource:    authz.APIResourceTypeKey,
			action:      authz.APIActionCreate,
			expectAllow: false,
			expectErr:   true,
		},
		{
			name:         "denied - tenant not loaded",
			makeCtx:      func() context.Context { return technicalCtx(tenantID, techRole) },
			setupHandler: func(_ *authz.Handler[authz.APIResourceType, authz.APIAction]) {},
			resource:     authz.APIResourceTypeKey,
			action:       authz.APIActionRead,
			expectAllow:  false,
			expectErr:    true,
		},
		{
			name: "error - missing tenant in context",
			makeCtx: func() context.Context {
				ctx := cmkcontext.InjectRequestID(context.Background(), "req-id")
				ctx, _ = cmkcontext.InjectTechnicalUserData(ctx, techRole)
				return ctx
			},
			setupHandler: func(h *authz.Handler[authz.APIResourceType, authz.APIAction]) {
				h.UpdateTechnicalUserData(authz.TenantID(tenantID))
			},
			resource:  authz.APIResourceTypeKey,
			action:    authz.APIActionRead,
			expectErr: true,
		},
		{
			name: "error - missing role in context",
			makeCtx: func() context.Context {
				ctx := cmkcontext.CreateTenantContext(context.Background(), tenantID)
				ctx = cmkcontext.InjectRequestID(ctx, "req-id")
				ctx = context.WithValue(ctx, constants.UserType, constants.TechnicalUser)
				// no role injected
				return ctx
			},
			setupHandler: func(h *authz.Handler[authz.APIResourceType, authz.APIAction]) {
				h.UpdateTechnicalUserData(authz.TenantID(tenantID))
			},
			resource:  authz.APIResourceTypeKey,
			action:    authz.APIActionRead,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newCheckAuthzHandler(t)
			tt.setupHandler(h)

			allowed, err := authz.CheckAuthz(tt.makeCtx(), h, tt.resource, tt.action)
			assert.Equal(t, tt.expectAllow, allowed)
			if tt.expectErr {
				assert.ErrorIs(t, err, authz.ErrAuthzDecision)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckAuthz_InternalUser(t *testing.T) {
	internalCtx := func(role constants.InternalRole) context.Context {
		ctx, _ := cmkcontext.InjectInternalUserData(context.Background(), role)
		return ctx
	}

	h := newCheckAuthzHandler(t)

	t.Run("allowed", func(t *testing.T) {
		ctx := internalCtx(constants.InternalRequestingHandlingRole)
		allowed, err := authz.CheckAuthz(ctx, h, authz.APIResourceTypeKey, authz.APIActionRead)
		assert.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("denied - role not in policy", func(t *testing.T) {
		ctx := internalCtx(constants.InternalTaskProcessingRole)
		_, err := authz.CheckAuthz(ctx, h, authz.APIResourceTypeKey, authz.APIActionRead)
		assert.ErrorIs(t, err, authz.ErrAuthzDecision)
	})
}
