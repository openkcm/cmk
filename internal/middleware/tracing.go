package middleware

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
)

func spanNameFormatter(basePath string) func(string, *http.Request) string {
	return func(operation string, r *http.Request) string {
		return operation + ":" + extractPattern(r.Pattern, basePath)
	}
}

func TracingMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return TracingMiddlewareWithBasePath(cfg, constants.BasePath)
}

func TracingMiddlewareWithBasePath(cfg *config.Config, basePath string) func(http.Handler) http.Handler {
	if !cfg.Telemetry.Traces.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		traceHeaderHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sc := trace.SpanFromContext(r.Context()).SpanContext()
			if sc.IsValid() {
				w.Header().Set("X-Trace-Id", sc.TraceID().String())
			}
			next.ServeHTTP(w, r)
		})

		return otelhttp.NewHandler(
			traceHeaderHandler,
			cfg.Application.Name,
			otelhttp.WithSpanNameFormatter(spanNameFormatter(basePath)),
		)
	}
}
