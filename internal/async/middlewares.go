package async

import (
	"context"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/openkcm/cmk/internal/config"
)

type Middleware func(asynq.HandlerFunc) asynq.HandlerFunc

func TracingMiddleware(cfg config.Config) func(next asynq.HandlerFunc) asynq.HandlerFunc {
	if !cfg.Telemetry.Traces.Enabled {
		return func(next asynq.HandlerFunc) asynq.HandlerFunc {
			return next
		}
	}

	return func(next asynq.HandlerFunc) asynq.HandlerFunc {
		return func(ctx context.Context, task *asynq.Task) error {
			tracer := otel.Tracer(cfg.Application.Name)
			ctx, span := tracer.Start(ctx, task.Type())
			defer span.End()

			err := next.ProcessTask(ctx, task)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}

			return err
		}
	}
}
