package daemon_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	ctr "github.com/openkcm/cmk/internal/controllers/cmk"
	"github.com/openkcm/cmk/internal/daemon"
)

var ErrWrite = errors.New("failed to write")

// failingResponseWriter is a response writer that fails on Write
type failingResponseWriter struct {
	http.ResponseWriter

	headerWritten bool
}

func (f *failingResponseWriter) Write([]byte) (int, error) {
	return 0, ErrWrite
}

func (f *failingResponseWriter) WriteHeader(statusCode int) {
	if !f.headerWritten {
		f.ResponseWriter.WriteHeader(statusCode)
	}
}

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

func TestServeMux_HandleFunc_PreventsBypassWithDuplicateBaseURL(t *testing.T) {
	mux := daemon.NewServeMux("/cmk/v1")

	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	// Should panic when pattern contains duplicate base URL
	// This tests that TrimPrefix (not Replace) is used, preventing bypass
	assert.Panics(t, func() {
		mux.HandleFunc("GET /cmk/v1/cmk/v1/keys", handler)
	}, "pattern with duplicate base URL should panic as it won't match authz registry")
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

func TestMuxWithSwagger(t *testing.T) {
	swagger, err := daemon.SetupSwagger()
	assert.NoError(t, err)

	t.Run("Should return ok", func(t *testing.T) {
		mux := daemon.NewServeMux("/cmk/v1", daemon.WithSwaggerUI(swagger))

		req := httptest.NewRequest(http.MethodGet, "/cmk/v1/swagger", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Should be available at /cmk/v1 if base url has tenant placeholder", func(t *testing.T) {
		mux := daemon.NewServeMux("/cmk/v1/{tenant}", daemon.WithSwaggerUI(swagger))

		req := httptest.NewRequest(http.MethodGet, "/cmk/v1/swagger", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Should handle write error gracefully", func(t *testing.T) {
		mux := daemon.NewServeMux("/cmk/v1", daemon.WithSwaggerUI(swagger))

		req := httptest.NewRequest(http.MethodGet, "/cmk/v1/swagger", nil)
		recorder := httptest.NewRecorder()
		w := &failingResponseWriter{ResponseWriter: recorder}

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	})
}
