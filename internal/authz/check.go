package authz

import (
	"context"
	"log/slog"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func CheckAuthz(
	ctx context.Context,
	authzHandler *Handler,
	resourceType ResourceTypeName,
	action Action,
) (bool, error) {
	tenant, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return false, errs.Wrap(ErrExtractTenantID, err)
	}

	identifier, err := cmkcontext.ExtractClientDataIdentifier(ctx)
	if err != nil {
		return false, errs.Wrap(ErrExtractClientData, err)
	}

	groups, err := cmkcontext.ExtractClientDataGroups(ctx)
	if err != nil {
		return false, errs.Wrap(ErrExtractClientData, err)
	}

	user := User{
		UserName: identifier,
		Groups:   groups,
	}

	log.Debug(
		ctx, "checking authorization request:", slog.String("user", user.UserName),
		slog.String("resourceType", string(resourceType)), slog.String("action", string(action)),
	)

	authzRequest, err := NewRequest(
		ctx,
		TenantID(tenant),
		user,
		resourceType,
		action,
	)
	if err != nil {
		return false, errs.Wrap(ErrCreateAuthzRequest, err)
	}

	allowed, err := authzHandler.IsAllowed(ctx, *authzRequest)
	if err != nil {
		return allowed, errs.Wrap(ErrAuthzDecision, err)
	}

	if !allowed {
		return allowed, ErrAuthzDecision
	}

	return allowed, nil
}
