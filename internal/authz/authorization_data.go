package authz

import (
	"errors"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
)

type BusinessUserAuthzKey struct {
	TenantID TenantID
	Group    string
}

type InternalUserAuthzKey struct {
	Role constants.InternalRole
}

type AuthorizationKey[TUser, TResourceTypeName, TAction comparable] struct {
	User             TUser
	ResourceTypeName TResourceTypeName
	Action           TAction
}

var ErrInvalidRole = errors.New("invalid role")

type AuthzData[TRole, TAuthzKey, TResourceTypeName, TAction comparable] struct {
	RolePolicies map[TRole][]BasePolicy[TRole, TResourceTypeName, TAction]
	AuthzKeys    map[AuthorizationKey[TAuthzKey, TResourceTypeName, TAction]]struct{}
}

type BusinessUserAuthzData[TResourceTypeName, TAction comparable] AuthzData[
	constants.BusinessRole, BusinessUserAuthzKey, TResourceTypeName, TAction]

// NewBusinessUserAuthzData creates and return a BusinessUserAuthzData. We have separate functions to add
// the entities, since these are dynamic
func NewBusinessUserAuthzData[TResourceTypeName, TAction comparable](
	rolePolicies map[constants.BusinessRole][]BasePolicy[constants.BusinessRole, TResourceTypeName, TAction]) (
	*BusinessUserAuthzData[TResourceTypeName, TAction], error) {
	authzData := &BusinessUserAuthzData[TResourceTypeName, TAction]{
		RolePolicies: rolePolicies,
		// The loader will add the authzkeys later
		AuthzKeys: make(map[AuthorizationKey[BusinessUserAuthzKey, TResourceTypeName, TAction]]struct{}),
	}
	return authzData, nil
}

func (l BusinessUserAuthzData[TResourceTypeName, TAction]) InitialiseAuthzKeys() {
	l.AuthzKeys = make(map[AuthorizationKey[BusinessUserAuthzKey, TResourceTypeName, TAction]]struct{})
}

func (l BusinessUserAuthzData[TResourceTypeName, TAction]) AddEntities(
	entities []Entity[constants.BusinessRole, BusinessUserEntity]) error {

	for _, entity := range entities {
		// entities with unknown roles are not authzKeys
		policies, ok := l.RolePolicies[entity.Role]
		if !ok {
			return errs.Wrap(ErrValidation, ErrInvalidRole)
		}

		for _, group := range entity.User.Groups {
			for _, policy := range policies {
				for _, resourceType := range policy.ResourceTypes {
					for _, action := range resourceType.Actions {
						key := AuthorizationKey[BusinessUserAuthzKey, TResourceTypeName, TAction]{
							User: BusinessUserAuthzKey{
								TenantID: entity.User.TenantID,
								Group:    group,
							},
							ResourceTypeName: resourceType.ID,
							Action:           action,
						}
						l.AuthzKeys[key] = struct{}{}
					}
				}
			}
		}
	}
	return nil
}

type InternalUserAuthzData[TResourceTypeName, TAction comparable] AuthzData[
	constants.InternalRole, InternalUserAuthzKey, TResourceTypeName, TAction]

// NewInternalUserAuthzData creates and return a BusinessUserAuthzData. There are no separate functions to add
// the entities, they are created on construction, since the policies and roles are all static
func NewInternalUserAuthzData[TResourceTypeName, TAction comparable](
	rolePolicies map[constants.InternalRole][]BasePolicy[constants.InternalRole, TResourceTypeName, TAction]) (
	*InternalUserAuthzData[TResourceTypeName, TAction], error) {
	// hold only authzKeys actions
	authzKeys := make(map[AuthorizationKey[InternalUserAuthzKey, TResourceTypeName, TAction]]struct{})

	for role, policies := range rolePolicies {
		for _, policy := range policies {
			for _, resourceType := range policy.ResourceTypes {
				for _, action := range resourceType.Actions {
					key := AuthorizationKey[InternalUserAuthzKey, TResourceTypeName, TAction]{
						User: InternalUserAuthzKey{
							Role: role,
						},
						ResourceTypeName: resourceType.ID,
						Action:           action,
					}
					authzKeys[key] = struct{}{}
				}
			}
		}
	}

	return &InternalUserAuthzData[TResourceTypeName, TAction]{
			AuthzKeys: authzKeys, RolePolicies: rolePolicies,
		},
		nil
}
