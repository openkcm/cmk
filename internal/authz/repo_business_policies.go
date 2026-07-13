package authz

import "github.com/openkcm/cmk/internal/constants"

var RepoBusinessPolicies = RolePolicies[constants.BusinessRole, RepoResourceType, RepoAction]{
	constants.TenantAuditorRole: {
		{
			ID: constants.AuditorPolicy,
			ResourceTypes: []Resource[RepoResourceType, RepoAction]{
				{
					Type: RepoResourceTypeCertificate,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeEvent,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeGroup,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeImportparam,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeKey,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeKeyconfiguration,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeKeystore,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeKeyversion,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeKeyLabel,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeResourceLabel,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeSystem,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeSystemProperty,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeTag,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeTenant,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeTenantconfig,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeWorkflow,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeWorkflowApprover,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
			},
		},
	},
	constants.KeyAdminRole: {
		{
			ID: constants.KeyAdminPolicy,
			ResourceTypes: []Resource[RepoResourceType, RepoAction]{
				{
					Type: RepoResourceTypeCertificate,
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
					Type: RepoResourceTypeEvent,
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
					Type: RepoResourceTypeGroup,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeImportparam,
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
					Type: RepoResourceTypeKey,
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
					Type: RepoResourceTypeKeyconfiguration,
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
					Type: RepoResourceTypeKeystore,
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
					Type: RepoResourceTypeKeyversion,
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
					Type: RepoResourceTypeKeyLabel,
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
					Type: RepoResourceTypeResourceLabel,
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
					Type: RepoResourceTypeSystem,
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
					Type: RepoResourceTypeSystemProperty,
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
					Type: RepoResourceTypeTag,
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
					Type: RepoResourceTypeTenant,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeTenantconfig,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate, // For setting keystore config
						RepoActionDelete, // For setting keystore config
					},
				},
				{
					Type: RepoResourceTypeWorkflow,
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
					Type: RepoResourceTypeWorkflowApprover,
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
		},
	},
	constants.TenantAdminRole: {
		{
			ID: constants.TenantAdminPolicy,
			ResourceTypes: []Resource[RepoResourceType, RepoAction]{
				{
					Type: RepoResourceTypeGroup,
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
					Type: RepoResourceTypeKeyconfiguration,
					Actions: []RepoAction{
						RepoActionFirst, // When deleting a group
					},
				},
				{
					Type: RepoResourceTypeTenant,
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
					Type: RepoResourceTypeTenantconfig,
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
		},
	},
}
