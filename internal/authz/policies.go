package authz

import "github.com/openkcm/cmk/internal/constants"

type (
	ResourceTypeName string
	ResourceType     struct {
		ID      ResourceTypeName
		Actions []Action
	}
)

const (
	ResourceTypeKeyConfiguration ResourceTypeName = "KeyConfiguration"
	ResourceTypeKey              ResourceTypeName = "Key"
	ResourceTypeSystem           ResourceTypeName = "System"
)

type Action string

const (
	ActionKeyConfigurationRead          Action = "KeyConfigurationRead"
	ActionKeyConfigurationCreate        Action = "KeyConfigurationCreate"
	ActionKeyConfigurationDelete        Action = "KeyConfigurationDelete"
	ActionKeyConfigurationSetPrimaryKey Action = "SetPrimaryKey"
	ActionKeyConfigurationUpdate        Action = "KeyConfigurationUpdate"
	ActionKeyConfigurationList          Action = "KeyConfigurationList"

	ActionKeyRead   Action = "KeyRead"
	ActionKeyCreate Action = "KeyCreate"
	ActionKeyDelete Action = "KeyDelete"
	ActionKeyUpdate Action = "KeyUpdate"
	ActionKeyRotate Action = "KeyRotate"
	ActionKeyList   Action = "KeyList"

	ActionSystemLink   Action = "SystemLink"
	ActionSystemUnlink Action = "SystemUnlink"
	ActionSystemList   Action = "SystemList"
)

var ResourceTypeActions = map[ResourceTypeName]map[Action]struct{}{
	ResourceTypeKeyConfiguration: {
		ActionKeyConfigurationRead:          {},
		ActionKeyConfigurationCreate:        {},
		ActionKeyConfigurationDelete:        {},
		ActionKeyConfigurationSetPrimaryKey: {},
		ActionKeyConfigurationUpdate:        {},
		ActionKeyConfigurationList:          {},
	},
	ResourceTypeKey: {
		ActionKeyRead:   {},
		ActionKeyCreate: {},
		ActionKeyDelete: {},
		ActionKeyUpdate: {},
		ActionKeyRotate: {},
		ActionKeyList:   {},
	},
	ResourceTypeSystem: {
		ActionSystemLink:   {},
		ActionSystemUnlink: {},
		ActionSystemList:   {},
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
	Roles         []constants.Role
	ResourceTypes []ResourceType
	Policies      []Policy
}

var PolicyData = policies{
	Roles: []constants.Role{
		constants.KeyAdminRole, constants.TenantAdminRole, constants.TenantAuditorRole,
	},
	ResourceTypes: []ResourceType{
		{
			ID: ResourceTypeKeyConfiguration,
			Actions: []Action{
				ActionKeyConfigurationRead, ActionKeyConfigurationCreate, ActionKeyConfigurationDelete,
			},
		},
		{
			ID: ResourceTypeKey,
			Actions: []Action{
				ActionKeyRead, ActionKeyCreate, ActionKeyDelete, ActionKeyUpdate,
			},
		},
		{
			ID: ResourceTypeSystem,
			Actions: []Action{
				ActionSystemLink, ActionSystemUnlink,
			},
		},
	},
	Policies: []Policy{
		{
			ID:   "AuditorPolicy",
			Role: constants.TenantAuditorRole,
			ResourceTypes: []ResourceType{
				{
					ID: ResourceTypeKeyConfiguration,
					Actions: []Action{
						ActionKeyConfigurationRead,
					},
				},
			},
		},
		{
			ID:   "AdminPolicy",
			Role: constants.KeyAdminRole,
			ResourceTypes: []ResourceType{
				{
					ID: ResourceTypeKeyConfiguration,
					Actions: []Action{
						ActionKeyConfigurationRead,
						ActionKeyConfigurationCreate,
						ActionKeyConfigurationDelete,
						ActionKeyConfigurationSetPrimaryKey,
						ActionKeyConfigurationUpdate,
						ActionKeyConfigurationList,
					},
				},
			},
		},
		{
			ID:   "TenantAdminPolicy",
			Role: constants.TenantAdminRole,
			ResourceTypes: []ResourceType{
				{
					ID: ResourceTypeSystem,
					Actions: []Action{
						ActionSystemList,
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
