package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/middleware"
)

var (
	cfgWithTracing = &config.Config{
		BaseConfig: commoncfg.BaseConfig{
			Application: commoncfg.Application{
				Name: "test-app",
			},
			Telemetry: commoncfg.Telemetry{
				Traces: commoncfg.Trace{
					Enabled: true,
				},
			},
		},
	}
)

func setupTestRecorder(t *testing.T) (*tracetest.SpanRecorder, func()) {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider()
	tp.RegisterSpanProcessor(recorder)

	original := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)

	return recorder, func() {
		otel.SetTracerProvider(original)
		_ = tp.Shutdown(t.Context())
	}
}

func TestTracingMiddleware(t *testing.T) {
	recorder, cleanup := setupTestRecorder(t)
	t.Cleanup(cleanup)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.TracingMiddleware(cfgWithTracing)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	mw(mux).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	spans := recorder.Ended()
	assert.Len(t, spans, 1)
	assert.Equal(t, "test-app:GET /healthz", spans[0].Name())
	assert.Equal(t, trace.SpanKindServer, spans[0].SpanKind())
}

func TestTracingMiddleware_Error(t *testing.T) {
	recorder, cleanup := setupTestRecorder(t)
	t.Cleanup(cleanup)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, err := w.Write([]byte("service unavailable"))
		assert.NoError(t, err)
	})

	mw := middleware.TracingMiddleware(cfgWithTracing)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	mw(mux).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	spans := recorder.Ended()
	assert.Len(t, spans, 1)
	assert.Equal(t, "test-app:GET /healthz", spans[0].Name())
	assert.Equal(t, trace.SpanKindServer, spans[0].SpanKind())
	assert.Equal(t, codes.Error, spans[0].Status().Code)
}

func TestTracingMiddleware_Disabled(t *testing.T) {
	recorder, cleanup := setupTestRecorder(t)
	t.Cleanup(cleanup)

	cfg := *cfgWithTracing
	cfg.Telemetry.Traces.Enabled = false

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.TracingMiddleware(&cfg)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	mw(next).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	spans := recorder.Ended()
	assert.Empty(t, spans)
}
