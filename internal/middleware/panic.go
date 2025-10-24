package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/openkcm/cmk/internal/api/write"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/log"
)

// PanicRecoveryMiddleware is a middleware that recovers from panics and logs them.
func PanicRecoveryMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func(ctx context.Context) {
				err := recover()
				if err != nil {
					//nolint:err113
					log.Error(ctx, "Panic Occurred", fmt.Errorf("%v", err),
						slog.String("stackTrace", string(debug.Stack())),
					)

					// Catch the panic and return 500
					write.ErrorResponse(ctx, w, apierrors.InternalServerErrorMessage())
				}
			}(r.Context())

			next.ServeHTTP(w, r)
		})
	}
}
