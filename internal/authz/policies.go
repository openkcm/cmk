package authz

import "github.tools.sap/kms/cmk/internal/constants"

type (
	ResourceTypeName string
	ResourceType     struct {
		ID      ResourceTypeName
		Actions []Action
	}
)

// all resource types which are used in policies
const (
	ResourceTypeKeyConfiguration ResourceTypeName = "KeyConfiguration"
	ResourceTypeKey              ResourceTypeName = "Key"
	ResourceTypeSystem           ResourceTypeName = "System"
	ResourceTypeWorkFlow         ResourceTypeName = "Workflow"
	ResourceTypeUserGroup        ResourceTypeName = "UserGroup"
	ResourceTypeTenant           ResourceTypeName = "Tenant"
)

type Action string

// all actions which are used in policies which can be performed on resource types
const (
	ActionRead             Action = "read"
	ActionList             Action = "list"
	ActionCreate           Action = "create"
	ActionUpdate           Action = "update"
	ActionDelete           Action = "delete"
	ActionKeyRotate        Action = "KeyRotate"
	ActionSystemModifyLink Action = "ModifySystemLink"
)

var ResourceTypeActions = map[ResourceTypeName]map[Action]struct{}{
	ResourceTypeKeyConfiguration: {
		ActionRead:   {},
		ActionCreate: {},
		ActionDelete: {},
		ActionUpdate: {},
		ActionList:   {},
	},
	ResourceTypeKey: {
		ActionRead:      {},
		ActionCreate:    {},
		ActionDelete:    {},
		ActionUpdate:    {},
		ActionKeyRotate: {},
		ActionList:      {},
	},
	ResourceTypeSystem: {
		ActionRead:             {},
		ActionList:             {},
		ActionSystemModifyLink: {},
	},
	ResourceTypeWorkFlow: {
		ActionRead:   {},
		ActionCreate: {},
		ActionDelete: {},
		ActionUpdate: {},
		ActionList:   {},
	},
	ResourceTypeUserGroup: {
		ActionRead:   {},
		ActionCreate: {},
		ActionDelete: {},
		ActionUpdate: {},
		ActionList:   {},
	},
	ResourceTypeTenant: {
		ActionRead:   {},
		ActionUpdate: {},
		ActionList:   {},
	},
}

var ActionResourceTypes map[Action]ResourceTypeName

var ValidRoles = make(map[constants.Role]struct{})

var RolePolicies = make(map[constants.Role][]Policy)

type Policy struct {
	ID            string
	Role          constants.Role
	ResourceTypes []ResourceType
}

type policies struct {
	Roles    []constants.Role
	Policies []Policy
}

var PolicyData = policies{
	Roles: []constants.Role{
		constants.KeyAdminRole, constants.TenantAdminRole, constants.TenantAuditorRole,
	},
	Policies: []Policy{
		{
			ID:   "AuditorPolicy",
			Role: constants.TenantAuditorRole,
			ResourceTypes: []ResourceType{
				{
					ID: ResourceTypeKeyConfiguration,
					Actions: []Action{
						ActionRead,
						ActionList,
					},
				},
				{
					ID: ResourceTypeKey,
					Actions: []Action{
						ActionRead,
						ActionList,
					},
				},
				{
					ID: ResourceTypeSystem,
					Actions: []Action{
						ActionList,
						ActionRead,
					},
				},
				{
					ID: ResourceTypeWorkFlow,
					Actions: []Action{
						ActionRead,
						ActionList,
					},
				},
				{
					ID: ResourceTypeUserGroup,
					Actions: []Action{
						ActionRead,
						ActionList,
					},
				},
				{
					ID: ResourceTypeTenant,
					Actions: []Action{
						ActionRead,
						ActionList,
					},
				},
			},
		},
		{
			ID:   "KeyAdminPolicy",
			Role: constants.KeyAdminRole,
			ResourceTypes: []ResourceType{
				{
					ID: ResourceTypeKeyConfiguration,
					Actions: []Action{
						ActionRead,
						ActionCreate,
						ActionDelete,
						ActionUpdate,
						ActionList,
					},
				},
				{
					ID: ResourceTypeKey,
					Actions: []Action{
						ActionRead,
						ActionCreate,
						ActionDelete,
						ActionUpdate,
						ActionKeyRotate,
						ActionList,
					},
				},
				{
					ID: ResourceTypeUserGroup,
					Actions: []Action{
						ActionRead,
						ActionList,
					},
				},
				{
					ID: ResourceTypeSystem,
					Actions: []Action{
						ActionSystemModifyLink,
						ActionList,
						ActionRead,
						ActionUpdate,
					},
				},
				{
					ID: ResourceTypeWorkFlow,
					Actions: []Action{
						ActionRead,
						ActionCreate,
						ActionDelete,
						ActionUpdate,
						ActionList,
					},
				},
			},
		},
		{
			ID:   "TenantAdminPolicy",
			Role: constants.TenantAdminRole,
			ResourceTypes: []ResourceType{
				{
					ID: ResourceTypeTenant,
					Actions: []Action{
						ActionRead,
						ActionUpdate,
						ActionList,
					},
				},
				{
					ID: ResourceTypeUserGroup,
					Actions: []Action{
						ActionRead,
						ActionCreate,
						ActionDelete,
						ActionUpdate,
						ActionList,
					},
				},
			},
		},
	},
}

func init() {
	ActionResourceTypes = make(map[Action]ResourceTypeName)

	for resourceType, actions := range ResourceTypeActions {
		for action := range actions {
			ActionResourceTypes[action] = resourceType
		}
	}

	for _, policy := range PolicyData.Policies {
		ValidRoles[policy.Role] = struct{}{}
	}

	// Index policies by role for fast lookup
	RolePolicies = make(map[constants.Role][]Policy)
	for _, policy := range PolicyData.Policies {
		RolePolicies[policy.Role] = append(RolePolicies[policy.Role], policy)
	}
}
