package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	multitenancyMiddleware "github.com/bartventer/gorm-multitenancy/middleware/nethttp/v8"

	"github.com/openkcm/cmk/internal/middleware"
)

func TestMultiTenancyMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		expectedTenant string
		expectedError  string
	}{
		{
			name:           "Valid Tenant",
			path:           "/cmk/v1/tenant123/resource",
			expectedTenant: "tenant123",
		},
		{
			name:          "Missing Tenant",
			path:          "/cmk/v1/abcd",
			expectedError: "invalid tenant or tenant not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock handler to capture tenant
			var capturedTenant string

			// Create a new HTTP handler
			handler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				tenant, err := r.Context().Value(multitenancyMiddleware.TenantKey).(string)
				if err {
					capturedTenant = tenant
				}
			})

			// Wrap handler with middleware
			middlewareFunc := middleware.InjectMultiTenancy()
			wrappedHandler := middlewareFunc(handler)

			// Create a new HTTP server
			router := http.NewServeMux()
			router.Handle("GET /cmk/v1/{tenant}/resource", wrappedHandler)
			router.Handle("GET /cmk/v1/abcd", wrappedHandler)

			ts := httptest.NewServer(router)
			defer ts.Close()

			// Create a new HTTP request
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL+tt.path, nil)
			assert.NoError(t, err)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			res := w.Result()

			// Validate results
			if tt.expectedError != "" {
				resBody, err := io.ReadAll(res.Body)
				assert.NoError(t, err)
				assert.Contains(t, string(resBody), tt.expectedError)
			} else {
				assert.Equal(t, tt.expectedTenant, capturedTenant)
			}
		})
	}
}
