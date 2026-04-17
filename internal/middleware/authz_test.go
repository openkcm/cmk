package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/controllers/cmk"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/middleware"
	"github.com/openkcm/cmk/internal/model"
	repomock "github.com/openkcm/cmk/internal/repo/mock"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func TestAuthzMiddleware_NoRestriction(t *testing.T) {
	ctr := &cmk.APIController{
		Repository: nil,
		Manager:    &manager.Manager{}, // Removed Authz reference
	}

	mw := middleware.AuthzMiddleware(ctr)

	// Create a dummy handler to wrap
	handler := mw(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/cmk/v1/{tenant}/unknown", nil)
	req.Pattern = "/cmk/v1/{tenant}/unknown"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}

	if !strings.Contains(rr.Body.String(), "Forbidden") {
		t.Errorf("expected Forbidden message, got %s", rr.Body.String())
	}
}

func TestAuthzMiddleware_RestrictionExists(t *testing.T) {
	ctx := testutils.CreateCtxWithTenant("tenant1")
	// Inject clientData2: identifier and groups
	identifier := "group1a" // must match a group in allowlist
	groups := []string{"group1a", "group1b"}
	ctx = testutils.InjectClientDataIntoContext(ctx, identifier, groups)
	ctx = cmkcontext.InjectRequestID(ctx, uuid.NewString())

	loader := SetupAuthzLoaderWithAllowList(t)

	ctr := &cmk.APIController{
		Repository:  nil,
		Manager:     &manager.Manager{},
		AuthzLoader: loader,
	}

	mw := middleware.AuthzMiddleware(ctr)

	// Create a dummy handler to wrap
	handler := mw(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/cmk/v1/{tenant}/keys", nil)
	req.Pattern = "GET /cmk/v1/{tenant}/keys"
	// Attach context with tenant ID and clientData
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestAuthzMiddleware_MissingAuthorizationHeader(t *testing.T) {
	ctr := &cmk.APIController{
		Repository: nil,
		Manager:    &manager.Manager{},
	}

	mw := middleware.AuthzMiddleware(ctr)

	handler := mw(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/cmk/v1/{tenant}/keys", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestAuthzMiddleware_InvalidPath(t *testing.T) {
	ctr := &cmk.APIController{
		Repository: nil,
		Manager:    &manager.Manager{},
	}

	mw := middleware.AuthzMiddleware(ctr)

	handler := mw(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/invalid/path", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestAuthzMiddleware_AllowedAPI(t *testing.T) {
	ctx := testutils.CreateCtxWithTenant("tenant1")
	ctr := &cmk.APIController{
		Repository: nil,
		Manager:    &manager.Manager{},
	}

	mw := middleware.AuthzMiddleware(ctr)

	handler := mw(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/cmk/v1/{tenant}/tenants", nil)
	req.Pattern = "GET /cmk/v1/{tenant}/tenants"
	rr := httptest.NewRecorder()
	req = req.WithContext(ctx)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestAuthzMiddleware_TenantWorkflowConfiguration(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		groupRole      constants.BusinessRole
		expectedStatus int
	}{
		// GET tests - all roles can read
		{
			name:           "GET: TenantAdmin can view workflow config",
			method:         http.MethodGet,
			groupRole:      constants.TenantAdminRole,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "GET: KeyAdmin can view workflow config",
			method:         http.MethodGet,
			groupRole:      constants.KeyAdminRole,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "GET: TenantAuditor can view workflow config",
			method:         http.MethodGet,
			groupRole:      constants.TenantAuditorRole,
			expectedStatus: http.StatusOK,
		},
		// PATCH tests - only TenantAdmin can edit
		{
			name:           "PATCH: TenantAdmin can edit workflow config",
			method:         http.MethodPatch,
			groupRole:      constants.TenantAdminRole,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "PATCH: KeyAdmin cannot edit workflow config",
			method:         http.MethodPatch,
			groupRole:      constants.KeyAdminRole,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "PATCH: TenantAuditor cannot edit workflow config",
			method:         http.MethodPatch,
			groupRole:      constants.TenantAuditorRole,
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantID := "test-tenant"
			groupIdentifier := "test-group-" + string(tt.groupRole)

			ctx := testutils.CreateCtxWithTenant(tenantID)
			ctx = testutils.InjectClientDataIntoContext(ctx, groupIdentifier, []string{groupIdentifier})
			ctx = cmkcontext.InjectRequestID(ctx, uuid.NewString())

			loader := setupAuthzLoaderWithRole(t, tenantID, groupIdentifier, tt.groupRole)

			ctr := &cmk.APIController{
				Repository:  nil,
				Manager:     &manager.Manager{},
				AuthzLoader: loader,
			}

			mw := middleware.AuthzMiddleware(ctr)
			handler := mw(
				http.HandlerFunc(
					func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
					},
				),
			)

			endpoint := "/cmk/v1/{tenant}/tenantConfigurations/workflow"
			pattern := tt.method + " " + endpoint
			req := httptest.NewRequest(tt.method, endpoint, nil)
			req.Pattern = pattern
			req = req.WithContext(ctx)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d, body: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

// Helper function to setup authz loader with a specific role
func setupAuthzLoaderWithRole(t *testing.T, tenantID, groupIdentifier string,
	role constants.BusinessRole) *authz_loader.AuthzLoader[authz.APIResourceTypeName, authz.APIAction] {
	t.Helper()

	r := repomock.NewInMemoryRepository()
	ctx := testutils.CreateCtxWithTenant(tenantID)

	err := r.Create(
		ctx, &model.Tenant{
			TenantModel: multitenancy.TenantModel{},
			ID:          tenantID,
			Status:      "Test",
		},
	)
	if err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	group := &model.Group{
		ID:            uuid.New(),
		IAMIdentifier: groupIdentifier,
		Role:          role,
	}
	err = r.Create(ctx, group)
	if err != nil {
		t.Fatalf("failed to create group: %v", err)
	}

	cfg := &config.Config{}

	loader := authz_loader.NewAPIAuthzLoader(ctx, r, cfg)

	return loader
}

// Go
func SetupAuthzLoaderWithAllowList(t *testing.T) *authz_loader.AuthzLoader[authz.APIResourceTypeName, authz.APIAction] {
	t.Helper()

	r := repomock.NewInMemoryRepository()
	tenants := []struct {
		tenantID string
		groups   []*model.Group
	}{
		{
			tenantID: "tenant1",
			groups: []*model.Group{
				{ID: uuid.New(), IAMIdentifier: "group1a", Role: constants.TenantAdminRole},
				{ID: uuid.New(), IAMIdentifier: "group1b", Role: constants.TenantAuditorRole},
				{ID: uuid.New(), IAMIdentifier: "group1c", Role: constants.KeyAdminRole},
			},
		},
	}

	for _, ts := range tenants {
		ctx := testutils.CreateCtxWithTenant(ts.tenantID)

		err := r.Create(
			ctx, &model.Tenant{
				TenantModel: multitenancy.TenantModel{},
				ID:          ts.tenantID,
				Status:      "Test",
			},
		)
		if err != nil {
			t.Fatalf("failed to create tenant: %v", err)
		}

		for _, g := range ts.groups {
			_ = r.Create(ctx, g)
		}
	}

	ctx := t.Context()
	cfg := &config.Config{}

	loader := authz_loader.NewAPIAuthzLoader(ctx, r, cfg)

	return loader
}
