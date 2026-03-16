package authz

import (
	"errors"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
)

type AuthorizationKey[TResourceTypeName, TAction comparable] struct {
	TenantID         TenantID
	UserGroup        string
	ResourceTypeName TResourceTypeName
	Action           TAction
}

var ErrInvalidRole = errors.New("invalid role")

type AllowList[TResourceTypeName, TAction comparable] struct {
	AuthzKeys map[AuthorizationKey[TResourceTypeName, TAction]]struct{}
	TenantIDs map[TenantID]struct{}
}

func NewAuthorizationData[TResourceTypeName, TAction comparable](
	entities []Entity,
	rolePolicies map[constants.Role][]BasePolicy[TResourceTypeName, TAction]) (
	*AllowList[TResourceTypeName, TAction], error) {
	// hold only authzKeys actions
	authzKeys := make(map[AuthorizationKey[TResourceTypeName, TAction]]struct{})
	// hold tenant IDs
	tenantIDs := make(map[TenantID]struct{})

	for _, entity := range entities {
		// entities with unknown roles are not authzKeys
		policies, ok := rolePolicies[entity.Role]
		if !ok {
			return nil, errs.Wrap(ErrValidation, ErrInvalidRole)
		}

		for _, group := range entity.UserGroups {
			for _, policy := range policies {
				for _, resourceType := range policy.ResourceTypes {
					for _, action := range resourceType.Actions {
						key := AuthorizationKey[TResourceTypeName, TAction]{
							TenantID:         entity.TenantID,
							UserGroup:        group,
							ResourceTypeName: resourceType.ID,
							Action:           action,
						}
						authzKeys[key] = struct{}{}
						// Add tenant ID to the list of tenant IDs in case it is not already present
						if _, exists := tenantIDs[entity.TenantID]; !exists {
							tenantIDs[entity.TenantID] = struct{}{}
						}
					}
				}
			}
		}
	}

	return &AllowList[TResourceTypeName, TAction]{AuthzKeys: authzKeys, TenantIDs: tenantIDs}, nil
}

func (l AllowList[TResourceTypeName, TAction]) ContainsTenant(id TenantID) bool {
	if _, ok := l.TenantIDs[id]; ok {
		return true
	}

	return false
}
