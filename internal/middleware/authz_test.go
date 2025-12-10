package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.tools.sap/kms/cmk/internal/authz"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/controllers/cmk"
	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/internal/middleware"
	"github.tools.sap/kms/cmk/internal/model"
	repomock "github.tools.sap/kms/cmk/internal/repo/mock"
	"github.tools.sap/kms/cmk/internal/testutils"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
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
	ctx = cmkcontext.InjectRequestID(ctx)

	engine := SetupAuthzEngineWithAllowList(t)

	ctr := &cmk.APIController{
		Repository:  nil,
		Manager:     &manager.Manager{},
		AuthzEngine: engine,
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

// Go
func SetupAuthzEngineWithAllowList(t *testing.T) *authz.Engine {
	t.Helper()

	repo := repomock.NewInMemoryRepository()
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

		err := repo.Create(
			ctx, &model.Tenant{
				TenantModel: multitenancy.TenantModel{},
				ID:          ts.tenantID,
				Region:      ts.tenantID,
				Status:      "Test",
			},
		)
		if err != nil {
			t.Fatalf("failed to create tenant: %v", err)
		}

		for _, g := range ts.groups {
			_ = repo.Create(ctx, g)
		}
	}

	cfg := &config.Config{}
	engine := authz.NewEngine(t.Context(), repo, cfg)

	return engine
}
