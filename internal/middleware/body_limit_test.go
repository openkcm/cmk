package middleware_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/middleware"
)

func TestRequestBodyLimitMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		body           io.Reader
		expectedStatus int
		expectBody     bool
	}{
		{
			name:           "body within limit is passed through",
			body:           strings.NewReader("small body"),
			expectedStatus: http.StatusOK,
			expectBody:     true,
		},
		{
			name:           "body exceeding 1 MiB is rejected",
			body:           bytes.NewReader(make([]byte, 1<<20+1)),
			expectedStatus: http.StatusRequestEntityTooLarge,
			expectBody:     false,
		},
		{
			name:           "body exactly at 1 MiB limit is passed through",
			body:           bytes.NewReader(make([]byte, 1<<20)),
			expectedStatus: http.StatusOK,
			expectBody:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
					return
				}
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodPost, "/test", tt.body)
			rr := httptest.NewRecorder()

			mid := middleware.RequestBodyLimitMiddleware()(next)
			mid.ServeHTTP(rr, req)

			require.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectBody {
				assert.NotEmpty(t, rr.Code)
			}
		})
	}
}
