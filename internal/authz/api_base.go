package authz

type (
	APIAction           string
	APIResourceTypeName string
	APIResourceType     struct {
		ID         APIResourceTypeName
		APIActions []APIAction
	}
)

// all resource types which are used in policies
const (
	APIResourceTypeKeyConfiguration APIResourceTypeName = "KeyConfiguration"
	APIResourceTypeKey              APIResourceTypeName = "Key"
	APIResourceTypeSystem           APIResourceTypeName = "System"
	APIResourceTypeWorkFlow         APIResourceTypeName = "Workflow"
	APIResourceTypeUserGroup        APIResourceTypeName = "UserGroup"
	APIResourceTypeTenant           APIResourceTypeName = "Tenant"
	APIResourceTypeTenantSettings   APIResourceTypeName = "TenantSettings"
	APIResourceTypeEvent            APIResourceTypeName = "Event"
	APIResourceTypeImportParams     APIResourceTypeName = "ImportParams"
	APIResourceTypeKeyStoreConfig   APIResourceTypeName = "KeyStoreConfig"
)

// all actions which are used in policies which can be performed on resource types
const (
	APIActionRead             APIAction = "read"
	APIActionCreate           APIAction = "create"
	APIActionUpdate           APIAction = "update"
	APIActionDelete           APIAction = "delete"
	APIActionKeyRotate        APIAction = "KeyRotate"
	APIActionSystemModifyLink APIAction = "ModifySystemLink"
)

var APIResourceTypeActions = map[APIResourceTypeName][]APIAction{
	APIResourceTypeKeyConfiguration: {
		APIActionRead,
		APIActionCreate,
		APIActionDelete,
		APIActionUpdate,
	},
	APIResourceTypeKey: {
		APIActionRead,
		APIActionCreate,
		APIActionDelete,
		APIActionUpdate,
		APIActionKeyRotate,
	},
	APIResourceTypeSystem: {
		APIActionRead,
		APIActionSystemModifyLink,
	},
	APIResourceTypeWorkFlow: {
		APIActionRead,
		APIActionCreate,
		APIActionDelete,
		APIActionUpdate,
	},
	APIResourceTypeTenantSettings: {
		APIActionRead,
		APIActionUpdate,
	},
	APIResourceTypeUserGroup: {
		APIActionRead,
		APIActionCreate,
		APIActionDelete,
		APIActionUpdate,
	},
	APIResourceTypeTenant: {
		APIActionRead,
		APIActionUpdate,
	},
}
