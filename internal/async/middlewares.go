package async

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/log"
)

type Middleware func(asynq.HandlerFunc) asynq.HandlerFunc

func TracingMiddleware(cfg config.Config) func(next asynq.HandlerFunc) asynq.HandlerFunc {
	if !cfg.Telemetry.Traces.Enabled {
		return func(next asynq.HandlerFunc) asynq.HandlerFunc {
			return next
		}
	}

	tracer := otel.Tracer(cfg.Application.Name)

	return func(next asynq.HandlerFunc) asynq.HandlerFunc {
		return func(ctx context.Context, task *asynq.Task) error {
			ctx, span := tracer.Start(ctx, task.Type())
			defer span.End()

			ctx = log.InjectTask(ctx, task)
			ctx = slogctx.With(
				ctx,
				slog.String(cfg.Logger.Formatter.Fields.OTel.TraceID,
					span.SpanContext().TraceID().String(),
				),
			)

			err := next.ProcessTask(ctx, task)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}

			return err
		}
	}
}
