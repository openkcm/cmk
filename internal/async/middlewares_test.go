package async_test

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/log"
)

type MockTaskHandler struct {
	err error
}

func (h *MockTaskHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	log.Info(ctx, "Processing mock task")
	return h.err
}

func (h *MockTaskHandler) TaskType() string {
	return "mock:task"
}

func setupTestSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider()
	tp.RegisterSpanProcessor(recorder)

	original := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(original)
		_ = tp.Shutdown(t.Context())
	})

	return recorder
}

func TestTracingMiddleware(t *testing.T) {
	cfgWithTracing := &config.Config{
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

	recorder := setupTestSpanRecorder(t)

	mw := async.TracingMiddleware(*cfgWithTracing)

	t.Run("should create a span for the task without error", func(t *testing.T) {
		handler := &MockTaskHandler{}
		wrappedProcessTask := mw(handler.ProcessTask)

		recorder.Reset()
		err := wrappedProcessTask(t.Context(), asynq.NewTask(handler.TaskType(), nil))
		assert.NoError(t, err)

		spans := recorder.Ended()
		assert.Len(t, spans, 1)
		assert.Equal(t, "mock:task", spans[0].Name())
		assert.Equal(t, trace.SpanKindInternal, spans[0].SpanKind())
	})

	t.Run("should create a span for the task with error", func(t *testing.T) {
		expectedErr := assert.AnError
		handler := &MockTaskHandler{err: expectedErr}
		wrappedProcessTask := mw(handler.ProcessTask)

		recorder.Reset()
		err := wrappedProcessTask(t.Context(), asynq.NewTask(handler.TaskType(), nil))
		assert.ErrorIs(t, err, expectedErr)

		spans := recorder.Ended()
		assert.Len(t, spans, 1)
		assert.Equal(t, "mock:task", spans[0].Name())
		assert.Equal(t, trace.SpanKindInternal, spans[0].SpanKind())
		assert.Equal(t, codes.Error, spans[0].Status().Code)
		assert.Equal(t, expectedErr.Error(), spans[0].Status().Description)
	})
}

func TestTracingMiddleware_Disabled(t *testing.T) {
	cfgWithoutTracing := &config.Config{
		BaseConfig: commoncfg.BaseConfig{
			Telemetry: commoncfg.Telemetry{
				Traces: commoncfg.Trace{
					Enabled: false,
				},
			},
		},
	}

	recorder := setupTestSpanRecorder(t)

	mw := async.TracingMiddleware(*cfgWithoutTracing)
	handler := &MockTaskHandler{}
	err := mw(handler.ProcessTask)(t.Context(), asynq.NewTask(handler.TaskType(), nil))
	assert.NoError(t, err)
	assert.Empty(t, recorder.Ended(), "Expected no spans to be recorded when tracing is disabled")
}
