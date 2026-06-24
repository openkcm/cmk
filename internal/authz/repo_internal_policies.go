package authz

import "github.com/openkcm/cmk/internal/constants"

var RepoInternalPolicies = RolePolicies[constants.InternalRole, RepoResourceType, RepoAction]{
	constants.InternalBusinessAuthzRole: {
		{
			ID: constants.InternalBusinessAuthzPolicy,
			ResourceTypes: []Resource[RepoResourceType, RepoAction]{
				{
					Type: RepoResourceTypeGroup,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeKeyconfiguration,
					Actions: []RepoAction{
						RepoActionCount,
					},
				},
			},
		},
	},
	constants.InternalTenantCLIRole: {
		{
			ID: constants.InternalTenantCLIPolicy,
			ResourceTypes: []Resource[RepoResourceType, RepoAction]{
				{
					Type: RepoResourceTypeTenant,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCreate,
						RepoActionDelete,
						RepoActionUpdate,
					},
				},
				{
					Type: RepoResourceTypeGroup,
					Actions: []RepoAction{
						RepoActionCreate,
					},
				},
			},
		},
	},
	constants.InternalEventReconcilerRole: {
		{
			ID: constants.InternalEventReconcilerPolicy,
			ResourceTypes: []Resource[RepoResourceType, RepoAction]{
				{
					Type: RepoResourceTypeTenant,
					Actions: []RepoAction{
						RepoActionFirst,
					},
				},
				{
					Type: RepoResourceTypeKey,
					Actions: []RepoAction{
						RepoActionFirst,
						RepoActionList,
						RepoActionUpdate,
					},
				},
				{
					Type: RepoResourceTypeSystem,
					Actions: []RepoAction{
						RepoActionFirst,
						RepoActionCount,
						RepoActionList,
						RepoActionUpdate,
					},
				},
				{
					// To get role-management client cert
					Type: RepoResourceTypeCertificate,
					Actions: []RepoAction{
						RepoActionFirst,
					},
				},
				{
					// To sync default keystore configuration
					Type: RepoResourceTypeTenantconfig,
					Actions: []RepoAction{
						RepoActionFirst,
						RepoActionDelete,
						RepoActionCreate,
					},
				},
				{
					// To update event error state and clean up completed events
					Type: RepoResourceTypeEvent,
					Actions: []RepoAction{
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
				{
					// To get the latest key version native ID
					Type: RepoResourceTypeKeyversion,
					Actions: []RepoAction{
						RepoActionFirst,
					},
				},
			},
		},
	},
	constants.InternalTenantProvisioningRole: {
		{
			ID: constants.InternalTenantProvisioningPolicy,
			ResourceTypes: []Resource[RepoResourceType, RepoAction]{
				{
					Type: RepoResourceTypeTenant,
					Actions: []RepoAction{
						RepoActionCreate,
						RepoActionUpdate,
						RepoActionDelete,
					},
				},
				{
					Type: RepoResourceTypeGroup,
					Actions: []RepoAction{
						RepoActionFirst,
						RepoActionCreate,
					},
				},
				{
					Type: RepoResourceTypeSystem,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionList,
						RepoActionFirst,
					},
				},
				{
					Type: RepoResourceTypeKey,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionList,
						RepoActionUpdate,
					},
				},
				{
					Type: RepoResourceTypeKeyconfiguration,
					Actions: []RepoAction{
						RepoActionFirst,
					},
				},
			},
		},
	},
	constants.InternalTaskProcessingRole: {
		{
			ID: constants.InternalTaskProcessingPolicy,
			ResourceTypes: []Resource[RepoResourceType, RepoAction]{
				{
					Type: RepoResourceTypeTenant,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionList,
					},
				},
			},
		},
	},
	constants.InternalTaskCertRotationRole: {
		{
			ID: constants.InternalTaskCertRotationPolicy,
			ResourceTypes: []Resource[RepoResourceType, RepoAction]{
				{
					Type: RepoResourceTypeCertificate,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCreate,
						RepoActionCount,
						RepoActionUpdate,
					},
				},
			},
		},
	},
	constants.InternalTaskWorkflowApproversRole: {
		{
			ID: constants.InternalTaskWorkflowApproversPolicy,
			ResourceTypes: []Resource[RepoResourceType, RepoAction]{
				{
					Type: RepoResourceTypeTenant,
					Actions: []RepoAction{
						RepoActionFirst,
					},
				},
				{
					Type: RepoResourceTypeTenantconfig,
					Actions: []RepoAction{
						RepoActionFirst,
					},
				},
				{
					Type: RepoResourceTypeWorkflow,
					Actions: []RepoAction{
						RepoActionUpdate,
						RepoActionFirst,
						// Delete+Create required by Set(WorkflowApproverGroup), which resolves its resource type to workflows
						RepoActionDelete,
						RepoActionCreate,
					},
				},
				{
					Type: RepoResourceTypeWorkflowApprover,
					Actions: []RepoAction{
						RepoActionCreate,
						RepoActionDelete,
						RepoActionCount,
						RepoActionList,
					},
				},
				{
					Type: RepoResourceTypeKey,
					Actions: []RepoAction{
						RepoActionFirst,
						// Update for setting the editable state
						RepoActionUpdate,
					},
				},
				{
					Type: RepoResourceTypeKeyversion,
					Actions: []RepoAction{
						RepoActionFirst,
					},
				},
				{
					Type: RepoResourceTypeGroup,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionList,
					},
				},
				{
					Type: RepoResourceTypeKeyconfiguration,
					Actions: []RepoAction{
						RepoActionFirst,
						RepoActionList,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeSystem,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionList,
						RepoActionFirst,
						// Update required by HandleTerminalWorkflow to clear system.UnderWorkflow on failure
						RepoActionUpdate,
					},
				},
				{
					Type: RepoResourceTypeSystemProperty,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionList,
					},
				},
				{
					Type: RepoResourceTypeEvent,
					Actions: []RepoAction{
						RepoActionFirst,
						RepoActionList,
						RepoActionCount,
					},
				},
			},
		},
	},
	constants.InternalTaskHYOKSyncRole: {
		{
			ID: constants.InternalTaskHYOKSyncPolicy,
			ResourceTypes: []Resource[RepoResourceType, RepoAction]{
				{
					Type: RepoResourceTypeKey,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionList,
						RepoActionUpdate,
					},
				},
				{
					Type: RepoResourceTypeCertificate,
					Actions: []RepoAction{
						RepoActionFirst,
						RepoActionCount,
						RepoActionCreate,
						RepoActionUpdate,
					},
				},
			},
		},
	},
	constants.InternalTaskKeystorePoolRole: {
		{
			ID: constants.InternalTaskKeystorePoolPolicy,
			ResourceTypes: []Resource[RepoResourceType, RepoAction]{
				{
					Type: RepoResourceTypeKeystore,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionCreate,
					},
				},
			},
		},
	},
	constants.InternalTaskSystemRefreshRole: {
		{
			ID: constants.InternalTaskSystemRefreshPolicy,
			ResourceTypes: []Resource[RepoResourceType, RepoAction]{
				{
					Type: RepoResourceTypeKeyconfiguration,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionCount,
					},
				},
				{
					Type: RepoResourceTypeSystem,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionList,
					},
				},
				{
					Type: RepoResourceTypeSystemProperty,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionList,
						RepoActionUpdate,
					},
				},
				{
					Type: RepoResourceTypeEvent,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionList,
					},
				},
			},
		},
	},
	constants.InternalTaskTenantRefreshRole: {
		{
			ID: constants.InternalTaskTenantRefreshPolicy,
			ResourceTypes: []Resource[RepoResourceType, RepoAction]{
				{
					Type: RepoResourceTypeTenant,
					Actions: []RepoAction{
						RepoActionUpdate,
					},
				},
			},
		},
	},
	constants.InternalTaskWorkflowCleanupRole: {
		{
			ID: constants.InternalTaskWorkflowCleanupPolicy,
			ResourceTypes: []Resource[RepoResourceType, RepoAction]{
				{
					Type: RepoResourceTypeTenantconfig,
					Actions: []RepoAction{
						RepoActionFirst,
					},
				},
				{
					Type: RepoResourceTypeWorkflow,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionCount,
						RepoActionDelete,
					},
				},
			},
		},
	},
	constants.InternalTaskWorkflowExpirationRole: {
		{
			ID: constants.InternalTaskWorkflowExpirationPolicy,
			ResourceTypes: []Resource[RepoResourceType, RepoAction]{
				{
					Type: RepoResourceTypeWorkflow,
					Actions: []RepoAction{
						RepoActionFirst,
						RepoActionList,
						RepoActionCount,
						RepoActionUpdate,
					},
				},
				{
					Type: RepoResourceTypeSystem,
					Actions: []RepoAction{
						RepoActionUpdate,
					},
				},
			},
		},
	},
}
