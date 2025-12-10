package authz

import (
	"context"
	"log/slog"

	"github.tools.sap/kms/cmk/internal/auditor"
	"github.tools.sap/kms/cmk/internal/log"
)

type Reason string

// LogDecision logs the authorization decision made for a request.
// It logs the request ID, tenant ID, resource type, action, decision, and reason.
// The decision is logged as an Info log if it is "Allow", otherwise as a Warn log.
// Additionally, it sends an audit log for unauthorized requests using the provided auditor.
func LogDecision(ctx context.Context, request Request, auditor *auditor.Auditor, isAllowed bool, reason Reason) {
	logFn := log.Warn

	if isAllowed { // Allow
		logFn = log.Info
	} else { // Deny
		// send audit log for unauthorized requests
		err := auditor.SendCmkUnauthorizedRequestAuditLog(ctx, string(request.ResourceTypeName), string(request.Action))
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
			slog.String("Resource", string(request.ResourceTypeName)),
			slog.String("Action", string(request.Action)),
			slog.String("Reason", string(reason)),
		),
	)
}
