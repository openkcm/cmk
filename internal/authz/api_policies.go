package authz

import (
	"github.com/openkcm/cmk/internal/constants"
)

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

var APIRolePolicies = make(map[constants.Role][]BasePolicy[APIResourceTypeName, APIAction])

type policies struct {
	Roles    []constants.Role
	Policies []BasePolicy[APIResourceTypeName, APIAction]
}

var PolicyData = policies{
	Roles: []constants.Role{
		constants.KeyAdminRole, constants.TenantAdminRole, constants.TenantAuditorRole,
	},
	Policies: []BasePolicy[APIResourceTypeName, APIAction]{
		NewPolicy(
			"AuditorPolicy",
			constants.TenantAuditorRole,
			[]BaseResourceType[APIResourceTypeName, APIAction]{
				{
					ID: APIResourceTypeKeyConfiguration,
					Actions: []APIAction{
						APIActionRead,
					},
				},
				{
					ID: APIResourceTypeKey,
					Actions: []APIAction{
						APIActionRead,
					},
				},
				{
					ID: APIResourceTypeSystem,
					Actions: []APIAction{
						APIActionRead,
					},
				},
				{
					ID: APIResourceTypeWorkFlow,
					Actions: []APIAction{
						APIActionRead,
					},
				},
				{
					ID: APIResourceTypeTenantSettings,
					Actions: []APIAction{
						APIActionRead,
					},
				},
				{
					ID: APIResourceTypeUserGroup,
					Actions: []APIAction{
						APIActionRead,
					},
				},
				{
					ID: APIResourceTypeTenant,
					Actions: []APIAction{
						APIActionRead,
					},
				},
			},
		),
		NewPolicy(
			"KeyAdminPolicy",
			constants.KeyAdminRole,
			[]BaseResourceType[APIResourceTypeName, APIAction]{
				{
					ID: APIResourceTypeKeyConfiguration,
					Actions: []APIAction{
						APIActionRead,
						APIActionCreate,
						APIActionDelete,
						APIActionUpdate,
					},
				},
				{
					ID: APIResourceTypeKey,
					Actions: []APIAction{
						APIActionRead,
						APIActionCreate,
						APIActionDelete,
						APIActionUpdate,
						APIActionKeyRotate,
					},
				},
				{
					ID: APIResourceTypeUserGroup,
					Actions: []APIAction{
						APIActionRead,
					},
				},
				{
					ID: APIResourceTypeSystem,
					Actions: []APIAction{
						APIActionSystemModifyLink,
						APIActionRead,
						APIActionUpdate,
					},
				},
				{
					ID: APIResourceTypeWorkFlow,
					Actions: []APIAction{
						APIActionRead,
						APIActionCreate,
						APIActionDelete,
						APIActionUpdate,
					},
				},
				{
					ID: APIResourceTypeTenantSettings,
					Actions: []APIAction{
						APIActionRead,
					},
				},
			},
		),
		NewPolicy(
			"TenantAdminPolicy",
			constants.TenantAdminRole,
			[]BaseResourceType[APIResourceTypeName, APIAction]{
				{
					ID: APIResourceTypeTenant,
					Actions: []APIAction{
						APIActionRead,
						APIActionUpdate,
					},
				},
				{
					ID: APIResourceTypeUserGroup,
					Actions: []APIAction{
						APIActionRead,
						APIActionCreate,
						APIActionDelete,
						APIActionUpdate,
					},
				},
				{
					ID: APIResourceTypeTenantSettings,
					Actions: []APIAction{
						APIActionRead,
						APIActionUpdate,
					},
				},
			},
		),
	},
}

func init() {
	// Index policies by role for fast lookup
	APIRolePolicies = make(map[constants.Role][]BasePolicy[APIResourceTypeName, APIAction])
	for _, policy := range PolicyData.Policies {
		APIRolePolicies[policy.Role] = append(APIRolePolicies[policy.Role], policy)
	}
}
