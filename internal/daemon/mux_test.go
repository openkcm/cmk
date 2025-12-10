package daemon_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	ctr "github.tools.sap/kms/cmk/internal/controllers/cmk"
	"github.tools.sap/kms/cmk/internal/daemon"
)

func TestServeMux_HandleFunc(t *testing.T) {
	mux := daemon.NewServeMux("/cmk/v1")

	called := false
	handler := func(w http.ResponseWriter, _ *http.Request) {
		called = true

		w.WriteHeader(http.StatusOK)
	}

	// Should not panic for registered pattern
	assert.NotPanics(t, func() {
		mux.HandleFunc("GET /cmk/v1/keys", handler)
	})

	// Should panic for unregistered pattern
	assert.Panics(t, func() {
		mux.HandleFunc("POST /unregistered", handler)
	})

	// Should panic for pattern without base path
	assert.NotPanics(t, func() {
		mux.HandleFunc("POST /keys", handler)
	})

	// Test that handler is called
	req := httptest.NewRequest(http.MethodGet, "/cmk/v1/keys", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCmkapiHandler(t *testing.T) {
	assert.NotPanics(t, func() {
		cmkapi.HandlerWithOptions(
			cmkapi.NewStrictHandlerWithOptions(
				&ctr.APIController{},
				[]cmkapi.StrictMiddlewareFunc{},
				cmkapi.StrictHTTPServerOptions{},
			),
			cmkapi.StdHTTPServerOptions{
				BaseRouter: daemon.NewServeMux(""),
			},
		)
	}, "some API patterns are not registered in authz")
}
