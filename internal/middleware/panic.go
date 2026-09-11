package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/openkcm/cmk/internal/api/cmk/write"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/log"
)

// PanicRecoveryMiddleware is a middleware that recovers from panics and logs them.
func PanicRecoveryMiddleware() func(http.Handler) http.Handler {
	return PanicRecoveryMiddlewareWithWriter(func(ctx context.Context, w http.ResponseWriter) {
		write.ErrorResponse(ctx, w, apierrors.InternalServerErrorMessage())
	})
}

type errorWriterFunc func(ctx context.Context, w http.ResponseWriter)

// PanicRecoveryMiddlewareWithWriter is a middleware that recovers from panics and logs them,
// using the provided errorWriter to write the 500 response.
func PanicRecoveryMiddlewareWithWriter(errorWriter errorWriterFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func(ctx context.Context) {
				err := recover()
				if err != nil {
					//nolint:err113
					log.Error(ctx, "Panic Occurred", fmt.Errorf("%v", err),
						slog.String("stackTrace", string(debug.Stack())),
					)

					errorWriter(ctx, w)
				}
			}(r.Context())

			next.ServeHTTP(w, r)
		})
	}
}
