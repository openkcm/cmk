package authz

type (
	BaseResourceType[TResourceTypeName, TAction comparable] struct {
		ID      TResourceTypeName
		Actions []TAction
	}
	BasePolicy[TRole, TResourceTypeName, TAction comparable] struct {
		ID            string
		Role          TRole
		ResourceTypes []BaseResourceType[TResourceTypeName, TAction]
	}
)

func NewPolicy[TRole, TResourceTypeName, TAction comparable](id string, role TRole,
	resourceTypes []BaseResourceType[TResourceTypeName, TAction]) BasePolicy[
	TRole, TResourceTypeName, TAction] {
	return BasePolicy[TRole, TResourceTypeName, TAction]{
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
