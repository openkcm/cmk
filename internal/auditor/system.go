package auditor

import (
	"context"

	"github.com/openkcm/common-sdk/pkg/auth"
	"go.opentelemetry.io/collector/pdata/plog"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"

	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// SendCmkOnboardingAuditLog sends an audit log for CMK onboarding
func (a *Auditor) SendCmkOnboardingAuditLog(ctx context.Context, systemID, cmkID, initiator string) error {
	if initiator != "" {
		ctx = cmkcontext.InjectBusinessUserData(ctx, &auth.ClientData{Identifier: initiator}, nil)
	}
	return a.sendEvent(ctx, func(metadata otlpaudit.EventMetadata) (plog.Logs, error) {
		return otlpaudit.NewCmkOnboardingEvent(metadata, cmkID, systemID)
	})
}

// SendCmkOffboardingAuditLog sends an audit log for CMK offboarding
func (a *Auditor) SendCmkOffboardingAuditLog(ctx context.Context, systemID, cmkID, initiator string) error {
	if initiator != "" {
		ctx = cmkcontext.InjectBusinessUserData(ctx, &auth.ClientData{Identifier: initiator}, nil)
	}
	return a.sendEvent(ctx, func(metadata otlpaudit.EventMetadata) (plog.Logs, error) {
		return otlpaudit.NewCmkOffboardingEvent(metadata, cmkID, systemID)
	})
}

// SendCmkSwitchAuditLog sends an audit log for CMK switching
func (a *Auditor) SendCmkSwitchAuditLog(ctx context.Context, systemID, cmkIDOld, cmkIDNew, initiator string) error {
	if initiator != "" {
		ctx = cmkcontext.InjectBusinessUserData(ctx, &auth.ClientData{Identifier: initiator}, nil)
	}
	return a.sendEvent(ctx, func(metadata otlpaudit.EventMetadata) (plog.Logs, error) {
		return otlpaudit.NewCmkSwitchEvent(metadata, systemID, cmkIDOld, cmkIDNew)
	})
}

// SendCmkTenantModificationAuditLog sends an audit log for CMK tenant modification
func (a *Auditor) SendCmkTenantModificationAuditLog(
	ctx context.Context,
	systemID, cmkID string,
	action otlpaudit.CmkAction,
) error {
	return a.sendEvent(ctx, func(metadata otlpaudit.EventMetadata) (plog.Logs, error) {
		return otlpaudit.NewCmkTenantModificationEvent(metadata, cmkID, systemID, action)
	})
}
