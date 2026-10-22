package authz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

var (
	ErrAuthzDecision            = errors.New("error making authorization decision")
	ErrAuthzUnauthorized        = errors.New("action authorized")
	ErrExtractBusinessUserData  = errors.New("error extracting client data from context")
	ErrExtractInternalUserData  = errors.New("error extracting internal data from context")
	ErrExtractTechnicalUserData = errors.New("error extracting technical data from context")
)

func CheckAuthz[
	Resource APIResourceType | RepoResourceType,
	Action APIAction | RepoAction,
](
	ctx context.Context,
	authzHandler *Handler[Resource, Action],
	resourceType Resource,
	action Action,
) (bool, error) {
	userType, err := cmkcontext.ExtractUserType(ctx)
	if err != nil {
		return false, errs.Wrap(ErrAuthzDecision, err)
	}

	var allowed bool
	switch userType {
	case string(constants.BusinessUser):
		allowed, err = checkBusinessUserAuthz(ctx, authzHandler, resourceType, action)
		if err != nil {
			return false, errs.Wrap(ErrAuthzDecision, err)
		}
	case string(constants.InternalUser):
		allowed, err = checkInternalUserAuthz(ctx, authzHandler, resourceType, action)
		if err != nil {
			return false, errs.Wrap(ErrAuthzDecision, err)
		}
	case string(constants.TechnicalUser):
		allowed, err = checkTechnicalUserAuthz(ctx, authzHandler, resourceType, action)
		if err != nil {
			return false, errs.Wrap(ErrAuthzDecision, err)
		}
	default:
		return false, errs.Wrap(ErrAuthzDecision, ErrNoAuthzForUserType)
	}

	if !allowed {
		return false, errs.Wrap(ErrAuthzDecision, ErrAuthzUnauthorized)
	}

	return allowed, nil
}

func runAuthzCheck[
	TUser UserRequest,
	Resource APIResourceType | RepoResourceType,
	Action APIAction | RepoAction,
](
	ctx context.Context,
	user TUser,
	resourceType Resource,
	action Action,
	isAllowed func(context.Context, Request[TUser, Resource, Action]) (bool, error),
) (bool, error) {
	log.Debug(
		ctx, "checking authorization request",
		slog.String("resourceType", fmt.Sprintf("%v", resourceType)),
		slog.String("action", fmt.Sprintf("%v", action)),
	)

	authzRequest, err := NewRequest(ctx, user, resourceType, action)
	if err != nil {
		return false, errs.Wrap(ErrCreateAuthzRequest, err)
	}

	allowed, err := isAllowed(ctx, *authzRequest)
	if err != nil {
		return allowed, errs.Wrap(ErrAuthzDecision, err)
	}

	return allowed, nil
}

func checkBusinessUserAuthz[
	Resource APIResourceType | RepoResourceType,
	Action APIAction | RepoAction,
](
	ctx context.Context,
	authzHandler *Handler[Resource, Action],
	resourceType Resource,
	action Action,
) (bool, error) {
	tenant, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return false, errs.Wrap(ErrExtractTenantID, err)
	}

	identifier, err := cmkcontext.ExtractBusinessUserDataIdentifier(ctx)
	if err != nil {
		return false, errs.Wrap(ErrExtractBusinessUserData, err)
	}

	groups, err := cmkcontext.ExtractBusinessUserDataGroups(ctx)
	if err != nil {
		return false, errs.Wrap(ErrExtractBusinessUserData, err)
	}

	user := BusinessUserRequest{TenantID: TenantID(tenant), UserName: identifier, Groups: groups}
	return runAuthzCheck(ctx, user, resourceType, action, authzHandler.IsBusinessUserAllowed)
}

func checkInternalUserAuthz[
	Resource APIResourceType | RepoResourceType,
	Action APIAction | RepoAction,
](
	ctx context.Context,
	authzHandler *Handler[Resource, Action],
	resourceType Resource,
	action Action,
) (bool, error) {
	role, err := cmkcontext.ExtractInternalRole(ctx)
	if err != nil {
		return false, errs.Wrap(ErrExtractInternalUserData, err)
	}

	user := InternalUserRequest{Role: role}
	return runAuthzCheck(ctx, user, resourceType, action, authzHandler.IsInternalUserAllowed)
}

func checkTechnicalUserAuthz[
	Resource APIResourceType | RepoResourceType,
	Action APIAction | RepoAction,
](
	ctx context.Context,
	authzHandler *Handler[Resource, Action],
	resourceType Resource,
	action Action,
) (bool, error) {
	tenant, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return false, errs.Wrap(ErrExtractTechnicalUserData, err)
	}

	role, err := cmkcontext.ExtractTechnicalRole(ctx)
	if err != nil {
		return false, errs.Wrap(ErrExtractTechnicalUserData, err)
	}

	user := TechnicalUserRequest{TenantID: TenantID(tenant), Role: role}
	return runAuthzCheck(ctx, user, resourceType, action, authzHandler.IsTechnicalUserAllowed)
}

func CheckInternalUserRole(ctx context.Context, role constants.InternalRole) error {
	ctxRole, err := cmkcontext.ExtractInternalRole(ctx)
	if err != nil {
		return errs.Wrap(ErrExtractInternalUserData, err)
	}

	if ctxRole != role {
		return ErrAuthzUnauthorized
	}

	return nil
}

func CheckInternalUserRoles(ctx context.Context,
	allowedRoles []constants.InternalRole,
) error {
	ctxRole, err := cmkcontext.ExtractInternalRole(ctx)
	if err != nil {
		return errs.Wrap(ErrExtractInternalUserData, err)
	}

	if !slices.Contains(allowedRoles, ctxRole) {
		return ErrAuthzUnauthorized
	}

	return nil
}
