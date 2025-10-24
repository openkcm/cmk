package authz

import (
	"context"
	"log/slog"

	"github.com/openkcm/cmk/internal/log"
)

type Reason string

// LogDecision logs the authorization decision made for a request.
// It logs the request ID, tenant ID, resource type, action, decision, and reason.
// The decision is logged as an Info log if it is "Allow", otherwise as a Warn log.
func LogDecision(ctx context.Context, request Request, isAllowed bool, reason Reason) {
	logFn := log.Warn

	if isAllowed { // Allow
		logFn = log.Info
	}

	// log the authorization isAllowed without user information
	// to avoid leaking sensitive information
	// the user information will only be logged within the audit log
	logFn(
		ctx,
		"Authorization Decision",
		slog.Group("Authorization",
			slog.Bool("Allowed", isAllowed),
			slog.String("Resource", string(request.ResourceTypeName)),
			slog.String("Action", string(request.Action)),
			slog.String("Reason", string(reason)),
		),
	)
}
