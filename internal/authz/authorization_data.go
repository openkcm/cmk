package authz

import (
	"errors"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
)

type BusinessUserCheck struct {
	TenantID TenantID
	Group    string
}

type InternalUserCheck struct {
	Role constants.InternalRole
}

type AuthorizationKey[
	User BusinessUserCheck | InternalUserCheck,
	Resource APIResourceType | RepoResourceType,
	Action APIAction | RepoAction,
] struct {
	User         User
	ResourceType Resource
	Action       Action
}

var ErrInvalidRole = errors.New("invalid role")

type AuthzData[
	Role constants.BusinessRole | constants.InternalRole,
	UserCheck BusinessUserCheck | InternalUserCheck,
	Resource APIResourceType | RepoResourceType,
	Action APIAction | RepoAction,
] struct {
	RolePolicies RolePolicies[Role, Resource, Action]
	AuthzKeys    map[AuthorizationKey[UserCheck, Resource, Action]]struct{}
}

type BusinessUserAuthzData[
	Resource APIResourceType | RepoResourceType,
	Action APIAction | RepoAction,
] AuthzData[constants.BusinessRole, BusinessUserCheck, Resource, Action]

type InternalUserAuthzData[
	Resource APIResourceType | RepoResourceType,
	Action APIAction | RepoAction,
] AuthzData[constants.InternalRole, InternalUserCheck, Resource, Action]

// NewBusinessUserAuthzData creates and return a BusinessUserAuthzData. We have separate functions to add
// the entities, since these are dynamic
func NewBusinessUserAuthzData[
	Resource APIResourceType | RepoResourceType,
	Action APIAction | RepoAction,
](
	rolePolicies RolePolicies[constants.BusinessRole, Resource, Action]) (
	*BusinessUserAuthzData[Resource, Action], error,
) {
	authzData := &BusinessUserAuthzData[Resource, Action]{
		RolePolicies: rolePolicies,
		// The loader will add the authzkeys later
		AuthzKeys: make(map[AuthorizationKey[BusinessUserCheck, Resource, Action]]struct{}),
	}
	return authzData, nil
}

func (l *BusinessUserAuthzData[ResourceType, Action]) InitialiseAuthzKeys() {
	l.AuthzKeys = make(map[AuthorizationKey[BusinessUserCheck, ResourceType, Action]]struct{})
}

func (l *BusinessUserAuthzData[ResourceType, Action]) AddUser(
	user map[constants.BusinessRole]*BusinessUser,
) error {
	for role, tenantAuth := range user {
		policies, ok := l.RolePolicies[role]
		if !ok {
			return errs.Wrap(ErrValidation, ErrInvalidRole)
		}

		for _, policy := range policies {
			for _, resource := range policy.ResourceTypes {
				for _, action := range resource.Actions {
					for _, group := range tenantAuth.Groups {
						key := AuthorizationKey[BusinessUserCheck, ResourceType, Action]{
							User: BusinessUserCheck{
								TenantID: tenantAuth.TenantID,
								Group:    group,
							},
							ResourceType: resource.Type,
							Action:       action,
						}
						l.AuthzKeys[key] = struct{}{}
					}
				}
			}
		}
	}

	return nil
}

// NewInternalUserAuthzData creates and return a InternalUserAuthzData.
// There are no separate functions to add the entities,
// they are created on construction, since the policies and roles are all static
func NewInternalUserAuthzData[
	Resource APIResourceType | RepoResourceType,
	Action APIAction | RepoAction,
](
	rolePolicies RolePolicies[constants.InternalRole, Resource, Action]) (
	*InternalUserAuthzData[Resource, Action], error,
) {
	// hold only authzKeys actions
	authzKeys := make(map[AuthorizationKey[InternalUserCheck, Resource, Action]]struct{})

	for role, policies := range rolePolicies {
		for _, policy := range policies {
			for _, resource := range policy.ResourceTypes {
				for _, action := range resource.Actions {
					key := AuthorizationKey[InternalUserCheck, Resource, Action]{
						User: InternalUserCheck{
							Role: role,
						},
						ResourceType: resource.Type,
						Action:       action,
					}
					authzKeys[key] = struct{}{}
				}
			}
		}
	}

	return &InternalUserAuthzData[Resource, Action]{
		AuthzKeys:    authzKeys,
		RolePolicies: rolePolicies,
	}, nil
}
