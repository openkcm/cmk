package log

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/hibiken/asynq"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk/internal/model"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func InjectRequest(ctx context.Context, r *http.Request) context.Context {
	requestID, _ := cmkcontext.GetRequestID(ctx)
	tenant, _ := cmkcontext.ExtractTenantID(ctx)

	return slogctx.With(ctx,
		slog.Group("Request",
			slog.String("RequestID", requestID),
			slog.String("Method", r.Method),
			slog.String("Host", r.Host),
			slog.String("Path", r.URL.Path),
		),
		slog.Group("Tenant",
			slog.String("ID", tenant),
		),
	)
}

func InjectTenant(ctx context.Context, tenant *model.Tenant) context.Context {
	return slogctx.With(ctx,
		slog.Group("Tenant",
			slog.String("ID", tenant.ID),
			slog.String("Region", tenant.Region),
			slog.String("Schema", tenant.SchemaName),
		),
	)
}

func InjectGroups(ctx context.Context, groups []*model.Group) context.Context {
	groupIAMIdentifiers := make([]string, len(groups))
	for i, group := range groups {
		groupIAMIdentifiers[i] = group.IAMIdentifier
	}

	return slogctx.With(ctx,
		slog.Group("Groups",
			slog.Any("IAMIdentifiers", groupIAMIdentifiers),
		),
	)
}

func InjectKey(ctx context.Context, key *model.Key) context.Context {
	return slogctx.With(ctx,
		slog.Group("Key",
			slog.String("ID", key.ID.String()),
		),
	)
}

func InjectSystem(ctx context.Context, sys *model.System) context.Context {
	return slogctx.With(ctx,
		slog.Group("System",
			slog.String("ID", sys.ID.String()),
			slog.String("Identifier", sys.Identifier),
			slog.String("Type", sys.Type),
			slog.String("Region", sys.Region),
		),
	)
}

func InjectTask(ctx context.Context, task *asynq.Task) context.Context {
	return slogctx.With(ctx,
		slog.Group("Task",
			slog.String("Type", task.Type()),
		),
	)
}

func InjectSystemEvent(
	ctx context.Context,
	event string,
) context.Context {
	return slogctx.With(ctx,
		slog.Group("Event",
			slog.String("Name", event),
		),
	)
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
