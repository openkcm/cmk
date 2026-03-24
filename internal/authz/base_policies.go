package authz

import "github.com/openkcm/cmk/internal/constants"

type (
	BaseResourceType[TResourceTypeName, TAction comparable] struct {
		ID      TResourceTypeName
		Actions []TAction
	}
	BasePolicy[TResourceTypeName, TAction comparable] struct {
		ID            string
		Role          constants.Role
		ResourceTypes []BaseResourceType[TResourceTypeName, TAction]
	}
)

func NewPolicy[TResourceTypeName, TAction comparable](id string, role constants.Role,
	resourceTypes []BaseResourceType[TResourceTypeName, TAction]) BasePolicy[
	TResourceTypeName, TAction] {
	return BasePolicy[TResourceTypeName, TAction]{
		ID:            id,
		Role:          role,
		ResourceTypes: resourceTypes,
	}
}

func NewResourceTypes[TResourceTypeName, TAction comparable](id TResourceTypeName, actions []TAction) BaseResourceType[
	TResourceTypeName, TAction] {
	return BaseResourceType[TResourceTypeName, TAction]{
		ID:      id,
		Actions: actions,
	}
}
