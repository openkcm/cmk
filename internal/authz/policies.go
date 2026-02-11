package authz

import "github.com/openkcm/cmk/internal/constants"

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
	ResourceTypeTenantSettings   ResourceTypeName = "TenantSettings"
)

type Action string

// all actions which are used in policies which can be performed on resource types
const (
	ActionRead             Action = "read"
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
	},
	ResourceTypeKey: {
		ActionRead:      {},
		ActionCreate:    {},
		ActionDelete:    {},
		ActionUpdate:    {},
		ActionKeyRotate: {},
	},
	ResourceTypeSystem: {
		ActionRead:             {},
		ActionSystemModifyLink: {},
	},
	ResourceTypeWorkFlow: {
		ActionRead:   {},
		ActionCreate: {},
		ActionDelete: {},
		ActionUpdate: {},
	},
	ResourceTypeTenantSettings: {
		ActionRead:   {},
		ActionUpdate: {},
	},
	ResourceTypeUserGroup: {
		ActionRead:   {},
		ActionCreate: {},
		ActionDelete: {},
		ActionUpdate: {},
	},
	ResourceTypeTenant: {
		ActionRead:   {},
		ActionUpdate: {},
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
					},
				},
				{
					ID: ResourceTypeKey,
					Actions: []Action{
						ActionRead,
					},
				},
				{
					ID: ResourceTypeSystem,
					Actions: []Action{
						ActionRead,
					},
				},
				{
					ID: ResourceTypeWorkFlow,
					Actions: []Action{
						ActionRead,
					},
				},
				{
					ID: ResourceTypeTenantSettings,
					Actions: []Action{
						ActionRead,
					},
				},
				{
					ID: ResourceTypeUserGroup,
					Actions: []Action{
						ActionRead,
					},
				},
				{
					ID: ResourceTypeTenant,
					Actions: []Action{
						ActionRead,
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
					},
				},
				{
					ID: ResourceTypeUserGroup,
					Actions: []Action{
						ActionRead,
					},
				},
				{
					ID: ResourceTypeSystem,
					Actions: []Action{
						ActionSystemModifyLink,
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
					},
				},
				{
					ID: ResourceTypeTenantSettings,
					Actions: []Action{
						ActionRead,
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
					},
				},
				{
					ID: ResourceTypeUserGroup,
					Actions: []Action{
						ActionRead,
						ActionCreate,
						ActionDelete,
						ActionUpdate,
					},
				},
				{
					ID: ResourceTypeTenantSettings,
					Actions: []Action{
						ActionRead,
						ActionUpdate,
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
