package middleware_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/middleware"
)

// TestLoggingMiddleware tests the logging middleware
func TestLoggingMiddleware(t *testing.T) {
	var buf bytes.Buffer

	logger := slog.New(slog.NewTextHandler(&buf, nil))
	slog.SetDefault(logger)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middlewareFunc := middleware.LoggingMiddleware()

	middlewareHandler := middlewareFunc(testHandler)

	const path = "/test"

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)

	rec := httptest.NewRecorder()

	middlewareHandler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	logOutput := buf.String()

	assertions := []string{
		"Request Completed",
		"Received Request",
		fmt.Sprintf("HttpStatus=%d", http.StatusOK),
	}

	for _, assertion := range assertions {
		assert.Contains(t, logOutput, assertion)
	}
}
