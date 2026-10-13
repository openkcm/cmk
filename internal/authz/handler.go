package authz

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
)

type TenantID string

type BusinessUser struct {
	TenantID TenantID
	Groups   []string
}

type Entity[
	Role constants.BusinessRole | constants.InternalRole,
	User BusinessUser,
] struct {
	User User
	Role Role
}

type Handler[
	Resource APIResourceType | RepoResourceType,
	Action APIAction | RepoAction,
] struct {
	InternalUserAuthzData InternalUserAuthzData[Resource, Action]
	BusinessUserAuthzData BusinessUserAuthzData[Resource, Action]

	Auditor *auditor.Auditor

	resourceActions map[Resource][]Action

	mu *sync.Mutex
}

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

func NewAuthorizationHandler[
	Resource APIResourceType | RepoResourceType,
	Action APIAction | RepoAction,
](
	auditor *auditor.Auditor,
	internalUserPolicies RolePolicies[constants.InternalRole, Resource, Action],
	businessUserPolicies RolePolicies[constants.BusinessRole, Resource, Action],
	resourceActions map[Resource][]Action,
	mu *sync.Mutex,
) (*Handler[Resource, Action], error) {
	internalUserAuthzData, err := NewInternalUserAuthzData(internalUserPolicies)
	if err != nil {
		return nil, err
	}

	businessUserAuthzData, err := NewBusinessUserAuthzData(businessUserPolicies)
	if err != nil {
		return nil, err
	}

	return &Handler[Resource, Action]{
		resourceActions:       resourceActions,
		BusinessUserAuthzData: *businessUserAuthzData,
		InternalUserAuthzData: *internalUserAuthzData,
		Auditor:               auditor,
		mu:                    mu,
	}, nil
}

func (as *Handler[Resource, Action]) ResetBusinessUserData() {
	as.BusinessUserAuthzData.InitialiseAuthzKeys()
}

func (as *Handler[Resource, Action]) UpdateBusinessUserData(
	user map[constants.BusinessRole]*BusinessUser,
) error {
	return as.BusinessUserAuthzData.AddUser(user)
}

// IsBusinessUserAllowed checks if the given Business User is allowed to perform
// the given Action on the given Resource
func (as *Handler[Resource, Action]) IsBusinessUserAllowed(
	ctx context.Context,
	request Request[BusinessUserRequest, Resource, Action],
) (bool, error) {
	// The AuthzKeys are updated by a background task
	// This needs a mutex to be concurrent safe
	as.mu.Lock()
	defer as.mu.Unlock()

	err := request.IsValidContext(ctx)
	if err != nil {
		LogDecision(ctx, request, as.Auditor, false, Reason(err.Error()))
		return false, errs.Wrap(ErrInvalidRequest, err)
	}

	err = as.isValidResourceAction(request.ResourceTypeName, request.Action)
	if err != nil {
		LogDecision(ctx, request, as.Auditor, false, Reason(err.Error()))
		return false, errs.Wrap(ErrInvalidRequest, ErrInvalidRequest)
	}

	for _, group := range request.User.Groups {
		reqData := AuthorizationKey[BusinessUserCheck, Resource, Action]{
			User: BusinessUserCheck{
				TenantID: request.User.TenantID,
				Group:    group,
			},
			ResourceType: request.ResourceTypeName,
			Action:       request.Action,
		}
		_, ok := as.BusinessUserAuthzData.AuthzKeys[reqData]

		if ok {
			// Allow
			LogDecision(ctx, request, as.Auditor, true, Reason(InfoAuthorizationPassed))
			return true, nil
		}
	}

	// If no matching policy is found, deny authorization
	LogDecision(ctx, request, as.Auditor, false, Reason(ErrAuthorizationDecision.Error()))

	return false, errs.Wrap(ErrAuthorizationDecision, ErrAuthorizationDenied)
}

// IsInternalUserAllowed checks if the given Business User is allowed to perform
// the given Action on the given Resource
func (as *Handler[Resource, Action]) IsInternalUserAllowed(
	ctx context.Context,
	request Request[InternalUserRequest, Resource, Action],
) (bool, error) {
	err := request.IsValidContext(ctx)
	if err != nil {
		LogDecision(ctx, request, as.Auditor, false, Reason(err.Error()))
		return false, errs.Wrap(ErrInvalidRequest, err)
	}

	err = as.isValidResourceAction(request.ResourceTypeName, request.Action)
	if err != nil {
		LogDecision(ctx, request, as.Auditor, false, Reason(err.Error()))
		return false, errs.Wrap(ErrInvalidRequest, ErrInvalidRequest)
	}

	reqData := AuthorizationKey[InternalUserCheck, Resource, Action]{
		User: InternalUserCheck{
			Role: request.User.Role,
		},
		ResourceType: request.ResourceTypeName,
		Action:       request.Action,
	}
	_, ok := as.InternalUserAuthzData.AuthzKeys[reqData]

	if ok {
		// Allow
		LogDecision(ctx, request, as.Auditor, true, Reason(InfoAuthorizationPassed))
		return true, nil
	}

	// If no matching policy is found, deny authorization
	// Deny
	LogDecision(ctx, request, as.Auditor, false, Reason(ErrAuthorizationDecision.Error()))

	return false, errs.Wrap(ErrAuthorizationDecision, ErrAuthorizationDenied)
}

// isValidResourceAction checks if user can trigger action on resource
func (as *Handler[Resource, Action]) isValidResourceAction(
	resource Resource,
	action Action,
) error {
	actions, ok := as.resourceActions[resource]
	if !ok {
		return errs.Wrapf(ErrResourceTypeInvalid, string(resource))
	}
	if !slices.Contains(actions, action) {
		return errs.Wrapf(ErrActionInvalidForResource, string(action))
	}
	return nil
}
