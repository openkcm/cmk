package authz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

var (
	ErrAuthzDecision       = errors.New("error making authorization decision")
	ErrAuthzUnauthorized   = errors.New("action authorized")
	ErrExtractClientData   = errors.New("error extracting client data from context")
	ErrExtractInternalData = errors.New("error extracting internal data from context")
)

func CheckAuthz[TResourceTypeName, TAction comparable](
	ctx context.Context,
	authzHandler *Handler[TResourceTypeName, TAction],
	resourceType TResourceTypeName,
	action TAction,
) (bool, error) {
	source, err := cmkcontext.ExtractSource(ctx)
	if err != nil {
		return false, errs.Wrap(ErrAuthzDecision, err)
	}

	var allowed bool
	switch source {
	case string(constants.BusinessSource):
		allowed, err = checkBusinessUserAuthz(ctx, authzHandler, resourceType, action)
		if err != nil {
			return false, errs.Wrap(ErrAuthzDecision, err)
		}
	case string(constants.InternalSource):
		allowed, err = checkInternalUserAuthz(ctx, authzHandler, resourceType, action)
		if err != nil {
			return false, errs.Wrap(ErrAuthzDecision, err)
		}
	default:
		return false, errs.Wrap(ErrAuthzDecision, ErrNoAuthzForSource)
	}

	if !allowed {
		return false, errs.Wrap(ErrAuthzDecision, ErrAuthzUnauthorized)
	}

	return allowed, nil
}

func checkBusinessUserAuthz[TResourceTypeName, TAction comparable](
	ctx context.Context,
	authzHandler *Handler[TResourceTypeName, TAction],
	resourceType TResourceTypeName,
	action TAction,
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

	user := BusinessUserRequest{
		TenantID: TenantID(tenant),
		UserName: identifier,
		Groups:   groups,
	}

	log.Debug(
		ctx, "checking authorization request:", slog.String("user", user.UserName),
		slog.String("resourceType", fmt.Sprintf("%v", resourceType)),
		slog.String("action", fmt.Sprintf("%v", action)),
	)

	authzRequest, err := NewRequest[BusinessUserRequest, TResourceTypeName, TAction](
		ctx,
		user,
		resourceType,
		action,
	)
	if err != nil {
		return false, errs.Wrap(ErrCreateAuthzRequest, err)
	}

	allowed, err := authzHandler.IsBusinessUserAllowed(ctx, *authzRequest)
	if err != nil {
		return allowed, errs.Wrap(ErrAuthzDecision, err)
	}

	return allowed, nil
}

func checkInternalUserAuthz[TResourceTypeName, TAction comparable](
	ctx context.Context,
	authzHandler *Handler[TResourceTypeName, TAction],
	resourceType TResourceTypeName,
	action TAction,
) (bool, error) {
	role, err := cmkcontext.ExtractInternalRole(ctx)
	if err != nil {
		return false, errs.Wrap(ErrExtractInternalData, err)
	}

	user := InternalUserRequest{
		Role: role,
	}

	log.Debug(
		ctx, "checking authorization request:", slog.String("user", string(user.Role)),
		slog.String("resourceType", fmt.Sprintf("%v", resourceType)),
		slog.String("action", fmt.Sprintf("%v", action)),
	)

	authzRequest, err := NewRequest[InternalUserRequest, TResourceTypeName, TAction](
		ctx,
		user,
		resourceType,
		action,
	)
	if err != nil {
		return false, errs.Wrap(ErrCreateAuthzRequest, err)
	}

	allowed, err := authzHandler.IsInternalUserAllowed(ctx, *authzRequest)
	if err != nil {
		return allowed, errs.Wrap(ErrAuthzDecision, err)
	}

	return allowed, nil
}
