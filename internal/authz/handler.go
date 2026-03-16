package authz

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type TenantID string

type Entity struct {
	TenantID   TenantID
	Role       constants.Role
	UserGroups []string
}
type Handler[TResourceTypeName, TAction comparable] struct {
	RolePolicies        map[constants.Role][]BasePolicy[TResourceTypeName, TAction]
	resourceTypeActions map[TResourceTypeName][]TAction
	validActions        map[TAction]struct{}
	Entities            []Entity
	AuthorizationData   AllowList[TResourceTypeName, TAction]
	Auditor             *auditor.Auditor
}

const EmptyTenantID = TenantID("")

var (
	ErrInvalidRequest        = errors.New("invalid request")
	ErrEmptyRequest          = errors.New("empty request")
	ErrAuthorizationDecision = errors.New("authorization decision error")
	ErrAuthorizationDenied   = errors.New("authorization denied")
	ErrWrongTenantID         = errors.New("wrong tenant ID in request")

	ErrExtractClientData  = errors.New("error extracting client data from context")
	ErrCreateAuthzRequest = errors.New("error creating authorization request")
	ErrExtractTenantID    = errors.New("error extracting tenant ID from context")
	ErrAuthzDecision      = errors.New("error making authorization decision")

	ErrActionInvalid            = errors.New("action is invalid")
	ErrResourceTypeInvalid      = errors.New("resource type is invalid")
	ErrActionInvalidForResource = errors.New("action is invalid for resource type")
)

var InfoAuthorizationPassed = "Authorization check passed"

func NewAuthorizationHandler[TResourceTypeName, TAction comparable](
	entities *[]Entity, auditor *auditor.Auditor,
	rolePolicies map[constants.Role][]BasePolicy[TResourceTypeName, TAction],
	resourceTypeActions map[TResourceTypeName][]TAction,
) (
	*Handler[TResourceTypeName, TAction], error) {
	authorizationData := &AllowList[TResourceTypeName, TAction]{}

	var err error

	// Create authorization data from entities
	if len(*entities) != 0 {
		authorizationData, err = NewAuthorizationData(*entities, rolePolicies)
		if err != nil {
			return nil, err
		}
	}

	validActions := map[TAction]struct{}{}

	for _, actions := range resourceTypeActions {
		for _, action := range actions {
			validActions[action] = struct{}{}
		}
	}

	return &Handler[TResourceTypeName, TAction]{
		RolePolicies:        rolePolicies,
		resourceTypeActions: resourceTypeActions,
		validActions:        validActions,
		Entities:            *entities,
		AuthorizationData:   *authorizationData,
		Auditor:             auditor,
	}, nil
}

// IsAllowed checks if the given User is allowed to perform the given Action on the given resource
func (as *Handler[TResourceTypeName, TAction]) IsAllowed(ctx context.Context,
	ar Request[TResourceTypeName, TAction]) (bool, error) {
	// Check if the request data is filled
	var emptyAction TAction
	var emptyResourceTypeName TResourceTypeName
	if ar.User.UserName == "" || ar.User.Groups == nil ||
		ar.ResourceTypeName == emptyResourceTypeName || ar.Action == emptyAction {
		// Deny
		LogDecision(ctx, ar, as.Auditor, false, Reason(ErrEmptyRequest.Error()))

		return false, errs.Wrap(ErrInvalidRequest, ErrEmptyRequest)
	}

	// Get the tenant from the context
	tenant, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		// Deny
		LogDecision(ctx, ar, as.Auditor, false, Reason(err.Error()))

		return false, errs.Wrap(ErrValidation, err)
	}

	if ar.TenantID != TenantID(tenant) {
		// Deny
		LogDecision(ctx, ar, as.Auditor, false, Reason(ErrWrongTenantID.Error()))

		return false, errs.Wrap(ErrAuthorizationDecision, ErrWrongTenantID)
	}

	err = as.isValidResourceAction(ar)
	if err != nil {
		// Deny
		LogDecision(ctx, ar, as.Auditor, false, Reason(ErrInvalidRequest.Error()))

		return false, errs.Wrap(ErrInvalidRequest, ErrInvalidRequest)
	}

	for _, group := range ar.User.Groups {
		reqData := AuthorizationKey[TResourceTypeName, TAction]{
			TenantID:         ar.TenantID,
			UserGroup:        group,
			ResourceTypeName: ar.ResourceTypeName,
			Action:           ar.Action,
		}
		_, ok := as.AuthorizationData.AuthzKeys[reqData]

		if ok {
			// Allow
			LogDecision(ctx, ar, as.Auditor, true, Reason(InfoAuthorizationPassed))
			return true, nil
		}
	}

	// If no matching policy is found, deny authorization
	// Deny
	LogDecision(ctx, ar, as.Auditor, false, Reason(ErrAuthorizationDecision.Error()))

	return false, errs.Wrap(ErrAuthorizationDecision, ErrAuthorizationDenied)
}

func (as *Handler[TResourceTypeName, TAction]) isValidResourceAction(
	ar Request[TResourceTypeName, TAction]) error {
	if _, exists := as.validActions[ar.Action]; !exists {
		return errs.Wrapf(ErrActionInvalid, fmt.Sprintf("%v", ar.Action))
	}

	if actions, resourceExists := as.resourceTypeActions[ar.ResourceTypeName]; resourceExists {
		if actionExists := slices.Contains(actions, ar.Action); !actionExists {
			return errs.Wrapf(ErrActionInvalidForResource, fmt.Sprintf("%v", ar.Action))
		}
	} else {
		return errs.Wrapf(ErrResourceTypeInvalid, fmt.Sprintf("%v", ar.ResourceTypeName))
	}

	return nil
}
