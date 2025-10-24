package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/middleware"
)

func TestPanicRecoveryMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		handler        http.Handler
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "no panic",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte("ok"))
				assert.NoError(t, err)
			}),
			expectedStatus: http.StatusOK,
			expectedBody:   "ok",
		},
		{
			name: "panic occurs",
			handler: http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				panic("something went wrong")
			}),
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rr := httptest.NewRecorder()

			mid := middleware.PanicRecoveryMiddleware()(tt.handler)
			mid.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			assert.Contains(t, rr.Body.String(), tt.expectedBody)
		})
	}
}
