package log

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/hibiken/asynq"

	slogctx "github.com/veqryn/slog-context"

	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func InjectRequest(ctx context.Context, r *http.Request) context.Context {
	requestID, _ := cmkcontext.GetRequestID(ctx)
	tenant, _ := cmkcontext.ExtractTenantID(ctx)

	return slogctx.With(ctx,
		slog.String("requestId", requestID),
		slog.String("tenantId", tenant),
		slog.Group("requestData",
			slog.String("method", r.Method),
			slog.String("host", r.Host),
			slog.String("path", r.URL.Path),
		),
	)
}

func InjectTask(ctx context.Context, task *asynq.Task) context.Context {
	return slogctx.With(ctx, slog.String("taskType", task.Type()))
}

func InjectSystemEvent(
	ctx context.Context,
	event string,
) context.Context {
	return slogctx.With(ctx, slog.String("eventName", event))
}

func ErrorAttr(err error) slog.Attr {
	return slog.Attr{
		Key:   slogctx.ErrKey,
		Value: slog.StringValue(err.Error()),
	}
}

func Debug(ctx context.Context, msg string, args ...slog.Attr) {
	slogctx.LogAttrs(ctx, slog.LevelDebug, msg, args...)
}

func Warn(ctx context.Context, msg string, args ...slog.Attr) {
	slogctx.LogAttrs(ctx, slog.LevelWarn, msg, args...)
}

func Info(ctx context.Context, msg string, args ...slog.Attr) {
	slogctx.LogAttrs(ctx, slog.LevelInfo, msg, args...)
}

func Error(ctx context.Context, msg string, err error, args ...slog.Attr) {
	args = append(args, slogctx.Err(err))

	slogctx.LogAttrs(ctx, slog.LevelError, msg, args...)
}
