package authz

import "github.com/openkcm/cmk/internal/constants"

type (
	RepoAction           string
	RepoResourceTypeName string
	RepoResourceType     struct {
		ID      RepoResourceTypeName
		Actions []RepoAction
	}
)

// all resource types which are used in policies
// These are linked to table names, so will require a migration if changed.
// Having this linkage ensures that tables are more coupled to the authz resource identifiers
const (
	RepoResourceTypeCertificate      RepoResourceTypeName = RepoResourceTypeName(constants.CertificateTable)
	RepoResourceTypeEvent            RepoResourceTypeName = RepoResourceTypeName(constants.EventTable)
	RepoResourceTypeGroup            RepoResourceTypeName = RepoResourceTypeName(constants.GroupTable)
	RepoResourceTypeImportparam      RepoResourceTypeName = RepoResourceTypeName(constants.ImportparamTable)
	RepoResourceTypeKey              RepoResourceTypeName = RepoResourceTypeName(constants.KeyTable)
	RepoResourceTypeKeyconfiguration RepoResourceTypeName = RepoResourceTypeName(constants.KeyconfigurationTable)
	RepoResourceTypeKeystore         RepoResourceTypeName = RepoResourceTypeName(constants.KeystoreTable)
	RepoResourceTypeKeyversion       RepoResourceTypeName = RepoResourceTypeName(constants.KeyVersionTable)
	RepoResourceTypeKeyLabel         RepoResourceTypeName = RepoResourceTypeName(constants.KeyLabelTable)
	RepoResourceTypeSystem           RepoResourceTypeName = RepoResourceTypeName(constants.SystemTable)
	RepoResourceTypeSystemProperty   RepoResourceTypeName = RepoResourceTypeName(constants.SystemPropertyTable)
	RepoResourceTypeTag              RepoResourceTypeName = RepoResourceTypeName(constants.TagTable)
	RepoResourceTypeTenant           RepoResourceTypeName = RepoResourceTypeName(constants.TenantTable)
	RepoResourceTypeTenantconfig     RepoResourceTypeName = RepoResourceTypeName(constants.TenantconfigTable)
	RepoResourceTypeWorkflow         RepoResourceTypeName = RepoResourceTypeName(constants.WorkflowTable)
	RepoResourceTypeWorkflowApprover RepoResourceTypeName = RepoResourceTypeName(constants.WorkflowApproverTable)
)

// all actions which are used in policies which can be performed on resource types
const (
	RepoActionList   RepoAction = "list"
	RepoActionFirst  RepoAction = "first"
	RepoActionCount  RepoAction = "count"
	RepoActionCreate RepoAction = "create"
	RepoActionUpdate RepoAction = "update"
	RepoActionDelete RepoAction = "delete"
)

var fullActionList = []RepoAction{
	RepoActionList,
	RepoActionFirst,
	RepoActionCount,
	RepoActionCreate,
	RepoActionUpdate,
	RepoActionDelete,
}

var RepoResourceTypeActions = map[RepoResourceTypeName][]RepoAction{
	RepoResourceTypeCertificate:      fullActionList,
	RepoResourceTypeEvent:            fullActionList,
	RepoResourceTypeGroup:            fullActionList,
	RepoResourceTypeImportparam:      fullActionList,
	RepoResourceTypeKey:              fullActionList,
	RepoResourceTypeKeyconfiguration: fullActionList,
	RepoResourceTypeKeystore:         fullActionList,
	RepoResourceTypeKeyversion:       fullActionList,
	RepoResourceTypeKeyLabel:         fullActionList,
	RepoResourceTypeSystem:           fullActionList,
	RepoResourceTypeSystemProperty:   fullActionList,
	RepoResourceTypeTag:              fullActionList,
	RepoResourceTypeTenant:           fullActionList,
	RepoResourceTypeTenantconfig:     fullActionList,
	RepoResourceTypeWorkflow:         fullActionList,
	RepoResourceTypeWorkflowApprover: fullActionList,
}

var RepoRolePolicies = make(map[constants.Role][]BasePolicy[RepoResourceTypeName, RepoAction])

type repoPolicies struct {
	Roles    []constants.Role
	Policies []BasePolicy[RepoResourceTypeName, RepoAction]
}

var RepoPolicyData = repoPolicies{
	Roles: []constants.Role{
		constants.KeyAdminRole, constants.TenantAdminRole, constants.TenantAuditorRole,
	},
	Policies: []BasePolicy[RepoResourceTypeName, RepoAction]{
		NewPolicy(
			"AuditorPolicy",
			constants.TenantAuditorRole,
			[]BaseResourceType[RepoResourceTypeName, RepoAction]{
				{
					ID: RepoResourceTypeCertificate,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeEvent,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeGroup,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeImportparam,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeKey,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeKeyconfiguration,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeKeystore,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeKeyversion,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeKeyLabel,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeSystem,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeSystemProperty,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeTag,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeTenant,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeTenantconfig,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeWorkflow,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeWorkflowApprover,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
			},
		),
		NewPolicy(
			"KeyAdminPolicy",
			constants.KeyAdminRole,
			[]BaseResourceType[RepoResourceTypeName, RepoAction]{
				{
					ID: RepoResourceTypeCertificate,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate,
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
				{
					ID: RepoResourceTypeEvent,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate,
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
				{
					ID: RepoResourceTypeGroup,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeImportparam,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate,
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
				{
					ID: RepoResourceTypeKey,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate,
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
				{
					ID: RepoResourceTypeKeyconfiguration,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate,
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
				{
					ID: RepoResourceTypeKeystore,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate,
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
				{
					ID: RepoResourceTypeKeyversion,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate,
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
				{
					ID: RepoResourceTypeKeyLabel,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate,
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
				{
					ID: RepoResourceTypeSystem,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate,
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
				{
					ID: RepoResourceTypeSystemProperty,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate,
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
				{
					ID: RepoResourceTypeTag,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate,
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
				{
					ID: RepoResourceTypeTenant,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeTenantconfig,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate, // For setting keystore config
						RepoActionDelete, // For setting keystore config
					},
				},
				{
					ID: RepoResourceTypeWorkflow,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate,
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
				{
					ID: RepoResourceTypeWorkflowApprover,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate,
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
			},
		),
		NewPolicy(
			"TenantAdminPolicy",
			constants.TenantAdminRole,
			[]BaseResourceType[RepoResourceTypeName, RepoAction]{
				{
					ID: RepoResourceTypeGroup,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate,
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
				{
					ID: RepoResourceTypeKeyconfiguration,
					Actions: []RepoAction{
						RepoActionFirst, // When deleting a group
					},
				},
				{
					ID: RepoResourceTypeTenant,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate,
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
				{
					ID: RepoResourceTypeTenantconfig,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate,
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
			},
		),
	},
}

func init() {
	// Index policies by role for fast lookup
	RepoRolePolicies = make(map[constants.Role][]BasePolicy[RepoResourceTypeName, RepoAction])
	for _, policy := range RepoPolicyData.Policies {
		RepoRolePolicies[policy.Role] = append(RepoRolePolicies[policy.Role], policy)
	}
}
