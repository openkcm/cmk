package middleware

import (
	"net/http"
)

// maxRequestBodyBytes is the maximum allowed size for inbound request bodies.
const maxRequestBodyBytes = 1 << 20 // 1 MiB

// RequestBodyLimitMiddleware caps inbound request body size to prevent memory
// exhaustion from oversized payloads.
func RequestBodyLimitMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
			next.ServeHTTP(w, r)
		})
	}
}
