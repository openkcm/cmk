package authz

import (
	"context"
	"log/slog"

	"github.com/openkcm/cmk/internal/log"
)

type Reason string

// LogDecision logs the authorization decision as a structured log entry.
// The decision is logged as Info if allowed, otherwise as Warn.
func LogDecision[TUser UserRequest, TResourceTypeName, TAction comparable](
	ctx context.Context, request Request[TUser, TResourceTypeName, TAction],
	isAllowed bool, reason Reason,
) {
	logFn := log.Warn
	if isAllowed {
		logFn = log.Info
	}
	// log the authorization IsAllowed without user information
	// to avoid leaking sensitive information
	// the user information will only be logged within the audit log
	logFn(
		ctx,
		"Authorization Decision",
		slog.Group(
			"Authorization",
			slog.Bool("Allowed", isAllowed),
			slog.String("Resource", request.GetResourceTypeNameString()),
			slog.String("Action", request.GetActionString()),
			slog.String("Reason", string(reason)),
		),
	)
}
