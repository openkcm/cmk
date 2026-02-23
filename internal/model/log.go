package model

import (
	"context"
	"log/slog"

	slogctx "github.com/veqryn/slog-context"

	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func LogInjectTenant(ctx context.Context, tenant *Tenant) context.Context {
	return slogctx.With(ctx,
		slog.String("tenantId", tenant.ID),
		slog.Group("tenantData",
			slog.String("region", tenant.Region),
			slog.String("schema", tenant.SchemaName),
		),
	)
}

func WithLogInjectTenant(tenant *Tenant) cmkcontext.Opt {
	return func(ctx context.Context) context.Context {
		return LogInjectTenant(ctx, tenant)
	}
}

func LogInjectGroups(ctx context.Context, groups []*Group) context.Context {
	groupIAMIdentifiers := make([]string, len(groups))
	for i, group := range groups {
		groupIAMIdentifiers[i] = group.IAMIdentifier
	}

	return slogctx.With(ctx,
		slog.Group("groups",
			slog.Any("iamIdentifiers", groupIAMIdentifiers),
		),
	)
}

func WithLogInjectGroups(groups []*Group) cmkcontext.Opt {
	return func(ctx context.Context) context.Context {
		return LogInjectGroups(ctx, groups)
	}
}

func LogInjectKey(ctx context.Context, key *Key) context.Context {
	return slogctx.With(ctx, slog.String("keyId", key.ID.String()))
}

func WithLogInjectKey(key *Key) cmkcontext.Opt {
	return func(ctx context.Context) context.Context {
		return LogInjectKey(ctx, key)
	}
}

func LogInjectSystem(ctx context.Context, sys *System) context.Context {
	return slogctx.With(ctx,
		slog.Group("systemData",
			slog.String("id", sys.ID.String()),
			slog.String("identifier", sys.Identifier),
			slog.String("type", sys.Type),
			slog.String("region", sys.Region),
		),
	)
}

func WithLogInjectSystem(sys *System) cmkcontext.Opt {
	return func(ctx context.Context) context.Context {
		return LogInjectSystem(ctx, sys)
	}
}
