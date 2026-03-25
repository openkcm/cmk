package middleware

import (
	"net/http"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
)

func spanNameFormatter(operation string, r *http.Request) string {
	return operation + ":" + strings.Replace(r.Pattern, constants.BasePath, "", 1)
}

func TracingMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	if !cfg.Telemetry.Traces.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, cfg.Application.Name, otelhttp.WithSpanNameFormatter(spanNameFormatter))
	}
}
