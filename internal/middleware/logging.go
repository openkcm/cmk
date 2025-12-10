package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.tools.sap/kms/cmk/internal/log"
)

// LoggingMiddleware logs the start and end of each request, along with the duration and status code.
func LoggingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := log.InjectRequest(r.Context(), r)
			r = r.WithContext(ctx)

			log.Info(ctx, "Received Request")

			start := time.Now()
			lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(lrw, r)

			duration := time.Since(start)

			log.Info(ctx, "Request Completed",
				slog.Int("HttpStatus", lrw.statusCode),
				slog.Duration("Duration", duration),
			)
		})
	}
}

// Custom ResponseWriter to capture status codes
type loggingResponseWriter struct {
	http.ResponseWriter

	statusCode int
}

// WriteHeader captures the status code
func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}
