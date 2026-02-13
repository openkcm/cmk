package model

import (
	"context"
	"log/slog"

	slogctx "github.com/veqryn/slog-context"
)

func LogInjectTenant(ctx context.Context, tenant *Tenant) context.Context {
	return slogctx.With(ctx,
		slog.String("tenantId", tenant.ID),
		slog.Group("tenantData",
			slog.String("region", tenant.Region),
			slog.String("schema", tenant.SchemaName),
			slog.String("name", tenant.Name),
		),
	)
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

func LogInjectKey(ctx context.Context, key *Key) context.Context {
	return slogctx.With(ctx, slog.String("keyId", key.ID.String()))
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
