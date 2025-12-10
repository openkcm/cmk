package middleware

import (
	"net/http"

	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

// InjectRequestID injects a RequestID into the context to be used by other middlewares
func InjectRequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(cmkcontext.InjectRequestID(r.Context())))
		})
	}
}
