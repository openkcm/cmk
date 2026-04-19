package authz

import (
	"context"
	"log/slog"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/log"
)

type Reason string

// LogDecision logs the authorization decision made for a request.
// It logs the request ID, tenant ID, resource type, action, decision, and reason.
// The decision is logged as an Info log if it is "Allow", otherwise as a Warn log.
// Additionally, it sends an audit log for unauthorized requests using the provided auditor.
func LogDecision[TUser UserRequest, TResourceTypeName, TAction comparable](
	ctx context.Context, request Request[TUser, TResourceTypeName, TAction],
	auditor *auditor.Auditor, isAllowed bool, reason Reason) {
	logFn := log.Warn

	if isAllowed { // Allow
		logFn = log.Info
	} else { // Deny
		// send audit log for unauthorized requests
		err := auditor.SendCmkUnauthorizedRequestAuditLog(ctx,
			request.GetResourceTypeNameString(), request.GetActionString())
		if err != nil {
			log.Error(ctx, "Failed to send audit log for CMK authorization check", err)
		}
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
