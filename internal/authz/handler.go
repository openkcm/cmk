package authz

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
)

type TenantID string

type BusinessUserEntity struct {
	TenantID TenantID
	Groups   []string
}

type Entity[TRole, TUser any] struct {
	User TUser
	Role TRole
}

type Handler[TResourceTypeName, TAction comparable] struct {
	InternalUserAuthzData InternalUserAuthzData[TResourceTypeName, TAction]
	BusinessUserAuthzData BusinessUserAuthzData[TResourceTypeName, TAction]

	Auditor *auditor.Auditor

	resourceTypeActions map[TResourceTypeName][]TAction
	validActions        map[TAction]struct{}
}

const EmptyTenantID = TenantID("")

var (
	ErrInvalidRequest        = errors.New("invalid request")
	ErrAuthorizationDecision = errors.New("authorization decision error")
	ErrAuthorizationDenied   = errors.New("authorization denied")

	ErrCreateAuthzRequest = errors.New("error creating authorization request")
	ErrExtractTenantID    = errors.New("error extracting tenant ID from context")

	ErrActionInvalid            = errors.New("action is invalid")
	ErrResourceTypeInvalid      = errors.New("resource type is invalid")
	ErrActionInvalidForResource = errors.New("action is invalid for resource type")
)

var InfoAuthorizationPassed = "Authorization check passed"

func NewAuthorizationHandler[TResourceTypeName, TAction comparable](
	auditor *auditor.Auditor,
	internalUserPolicies map[constants.InternalRole][]BasePolicy[constants.InternalRole, TResourceTypeName, TAction],
	businessUserPolicies map[constants.BusinessRole][]BasePolicy[constants.BusinessRole, TResourceTypeName, TAction],
	resourceTypeActions map[TResourceTypeName][]TAction,
) (*Handler[TResourceTypeName, TAction], error) {
	internalUserAuthzData, err := NewInternalUserAuthzData(internalUserPolicies)
	if err != nil {
		return nil, err
	}

	businessUserAuthzData, err := NewBusinessUserAuthzData(businessUserPolicies)
	if err != nil {
		return nil, err
	}

	validActions := map[TAction]struct{}{}

	for _, actions := range resourceTypeActions {
		for _, action := range actions {
			validActions[action] = struct{}{}
		}
	}

	return &Handler[TResourceTypeName, TAction]{
		resourceTypeActions:   resourceTypeActions,
		validActions:          validActions,
		BusinessUserAuthzData: *businessUserAuthzData,
		InternalUserAuthzData: *internalUserAuthzData,
		Auditor:               auditor,
	}, nil
}

func (as *Handler[TResourceTypeName, TAction]) ResetBusinessUserData() {
	as.BusinessUserAuthzData.InitialiseAuthzKeys()
}

func (as *Handler[TResourceTypeName, TAction]) UpdateBusinessUserData(
	entities []Entity[constants.BusinessRole, BusinessUserEntity]) error {
	return as.BusinessUserAuthzData.AddEntities(entities)
}

// IsBusinessUserAllowed checks if the given Business User is allowed to perform
// the given Action on the given Resource
func (as *Handler[TResourceTypeName, TAction]) IsBusinessUserAllowed(ctx context.Context,
	ar Request[BusinessUserRequest, TResourceTypeName, TAction]) (bool, error) {
	err := ar.IsValidContext(ctx)
	if err != nil {
		LogDecision[BusinessUserRequest, TResourceTypeName, TAction](
			ctx, ar, as.Auditor, false, Reason(err.Error()))
		return false, errs.Wrap(ErrInvalidRequest, err)
	}

	err = as.isValidResourceAction(ar.ResourceTypeName, ar.Action)
	if err != nil {
		LogDecision[BusinessUserRequest, TResourceTypeName, TAction](
			ctx, ar, as.Auditor, false, Reason(err.Error()))
		return false, errs.Wrap(ErrInvalidRequest, ErrInvalidRequest)
	}

	for _, group := range ar.User.Groups {
		reqData := AuthorizationKey[BusinessUserAuthzKey, TResourceTypeName, TAction]{
			User: BusinessUserAuthzKey{
				TenantID: ar.User.TenantID,
				Group:    group,
			},
			ResourceTypeName: ar.ResourceTypeName,
			Action:           ar.Action,
		}
		_, ok := as.BusinessUserAuthzData.AuthzKeys[reqData]

		if ok {
			// Allow
			LogDecision[BusinessUserRequest, TResourceTypeName, TAction](
				ctx, ar, as.Auditor, true, Reason(InfoAuthorizationPassed))
			return true, nil
		}
	}

	// If no matching policy is found, deny authorization
	// Deny
	LogDecision[BusinessUserRequest, TResourceTypeName, TAction](
		ctx, ar, as.Auditor, false, Reason(ErrAuthorizationDecision.Error()))

	return false, errs.Wrap(ErrAuthorizationDecision, ErrAuthorizationDenied)
}

// IsBusinessUserAllowed checks if the given Business User is allowed to perform
// the given Action on the given Resource
func (as *Handler[TResourceTypeName, TAction]) IsInternalUserAllowed(ctx context.Context,
	ar Request[InternalUserRequest, TResourceTypeName, TAction]) (bool, error) {
	err := ar.IsValidContext(ctx)
	if err != nil {
		LogDecision[InternalUserRequest, TResourceTypeName, TAction](
			ctx, ar, as.Auditor, false, Reason(err.Error()))
		return false, errs.Wrap(ErrInvalidRequest, err)
	}

	err = as.isValidResourceAction(ar.ResourceTypeName, ar.Action)
	if err != nil {
		LogDecision[InternalUserRequest, TResourceTypeName, TAction](
			ctx, ar, as.Auditor, false, Reason(err.Error()))
		return false, errs.Wrap(ErrInvalidRequest, ErrInvalidRequest)
	}

	reqData := AuthorizationKey[InternalUserAuthzKey, TResourceTypeName, TAction]{
		User: InternalUserAuthzKey{
			Role: ar.User.Role,
		},
		ResourceTypeName: ar.ResourceTypeName,
		Action:           ar.Action,
	}
	_, ok := as.InternalUserAuthzData.AuthzKeys[reqData]

	if ok {
		// Allow
		LogDecision[InternalUserRequest, TResourceTypeName, TAction](
			ctx, ar, as.Auditor, true, Reason(InfoAuthorizationPassed))
		return true, nil
	}

	// If no matching policy is found, deny authorization
	// Deny
	LogDecision[InternalUserRequest, TResourceTypeName, TAction](
		ctx, ar, as.Auditor, false, Reason(ErrAuthorizationDecision.Error()))

	return false, errs.Wrap(ErrAuthorizationDecision, ErrAuthorizationDenied)
}

func (as *Handler[TResourceTypeName, TAction]) isValidResourceAction(
	resourceTypeName TResourceTypeName, action TAction) error {
	if _, exists := as.validActions[action]; !exists {
		return errs.Wrapf(ErrActionInvalid, fmt.Sprintf("%v", action))
	}

	if actions, resourceExists := as.resourceTypeActions[resourceTypeName]; resourceExists {
		if actionExists := slices.Contains(actions, action); !actionExists {
			return errs.Wrapf(ErrActionInvalidForResource, fmt.Sprintf("%v", action))
		}
	} else {
		return errs.Wrapf(ErrResourceTypeInvalid, fmt.Sprintf("%v", resourceTypeName))
	}

	return nil
}
