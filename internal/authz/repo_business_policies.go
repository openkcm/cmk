package authz

import "github.com/openkcm/cmk/internal/constants"

var RepoBusinessPolicies = make(map[constants.BusinessRole][]BasePolicy[constants.BusinessRole,
	RepoResourceTypeName, RepoAction])

type repoBusinessPolicies struct {
	Roles    []constants.BusinessRole
	Policies []BasePolicy[constants.BusinessRole, RepoResourceTypeName, RepoAction]
}

var BusinessRepoPolicyData = repoBusinessPolicies{
	Roles: []constants.BusinessRole{
		constants.KeyAdminRole, constants.TenantAdminRole, constants.TenantAuditorRole,
	},
	Policies: []BasePolicy[constants.BusinessRole, RepoResourceTypeName, RepoAction]{
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
	RepoBusinessPolicies = make(map[constants.BusinessRole][]BasePolicy[constants.BusinessRole,
		RepoResourceTypeName, RepoAction])
	for _, policy := range BusinessRepoPolicyData.Policies {
		RepoBusinessPolicies[policy.Role] = append(RepoBusinessPolicies[policy.Role], policy)
	}
}
