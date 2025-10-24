package authz

import (
	"errors"

	"github.com/openkcm/cmk-core/internal/errs"
)

type AuthorizationKey struct {
	TenantID         TenantID
	UserGroup        UserGroup
	ResourceTypeName ResourceTypeName
	Action           Action
}

var (
	ErrInvalidRole = errors.New("invalid role")
)

type AllowList struct {
	AuthzKeys map[AuthorizationKey]struct{}
	TenantIDs map[TenantID]struct{}
}

func NewAuthorizationData(entities []Entity) (*AllowList, error) {
	// hold only authzKeys actions
	authzKeys := make(map[AuthorizationKey]struct{})
	// hold tenant IDs
	tenantIDs := make(map[TenantID]struct{})

	for _, entity := range entities {
		// unknown roles are not authzKeys
		if _, ok := ValidRoles[entity.Role]; !ok {
			return nil, errs.Wrap(ErrValidation, ErrInvalidRole)
		}

		// entities with unknown roles are not authzKeys
		policies, ok := RolePolicies[entity.Role]
		if !ok {
			return nil, errs.Wrap(ErrValidation, ErrInvalidRole)
		}

		for _, group := range entity.UserGroups {
			for _, policy := range policies {
				for _, resourceType := range policy.ResourceTypes {
					for _, action := range resourceType.Actions {
						key := AuthorizationKey{
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

	return &AllowList{AuthzKeys: authzKeys, TenantIDs: tenantIDs}, nil
}

func (l AllowList) ContainsTenant(id TenantID) bool {
	if _, ok := l.TenantIDs[id]; ok {
		return true
	}

	return false
}
