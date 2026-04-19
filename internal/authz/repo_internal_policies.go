package authz

import "github.com/openkcm/cmk/internal/constants"

var RepoInternalPolicies = make(map[constants.InternalRole][]BasePolicy[constants.InternalRole,
	RepoResourceTypeName, RepoAction])

type internalRepoPolicies struct {
	Roles    []constants.InternalRole
	Policies []BasePolicy[constants.InternalRole, RepoResourceTypeName, RepoAction]
}

var InternalRepoPolicyData = internalRepoPolicies{
	Roles: []constants.InternalRole{
		constants.InternalTenantProvisioningRole,
	},
	Policies: []BasePolicy[constants.InternalRole, RepoResourceTypeName, RepoAction]{
		NewPolicy(
			"InternalBusinessAuthz",
			constants.InternalBusinessAuthzRole,
			[]BaseResourceType[RepoResourceTypeName, RepoAction]{
				{
					ID: RepoResourceTypeGroup,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeKeyconfiguration,
					Actions: []RepoAction{
						RepoActionCount,
					},
				},
			},
		),
		NewPolicy(
			"InternalTenantProvisioning",
			constants.InternalTenantProvisioningRole,
			[]BaseResourceType[RepoResourceTypeName, RepoAction]{
				{
					ID: RepoResourceTypeTenant,
					Actions: []RepoAction{
						RepoActionCreate,
					},
				},
				{
					ID: RepoResourceTypeGroup,
					Actions: []RepoAction{
						RepoActionCreate,
					},
				},
			},
		),
		NewPolicy(
			"InternalTaskProcessing",
			constants.InternalTaskProcessingRole,
			[]BaseResourceType[RepoResourceTypeName, RepoAction]{
				{
					ID: RepoResourceTypeTenant,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionList,
					},
				},
			},
		),
		NewPolicy(
			"InternalTaskCertRotation",
			constants.InternalTaskCertRotationRole,
			[]BaseResourceType[RepoResourceTypeName, RepoAction]{
				{
					ID: RepoResourceTypeCertificate,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionFirst,
						RepoActionCreate,
						RepoActionCount,
						RepoActionUpdate,
					},
				},
			},
		),
		NewPolicy(
			"InternalTaskWorkflowApprovers",
			constants.InternalTaskWorkflowApproversRole,
			[]BaseResourceType[RepoResourceTypeName, RepoAction]{
				{
					ID: RepoResourceTypeTenant,
					Actions: []RepoAction{
						RepoActionFirst,
					},
				},
				{
					ID: RepoResourceTypeTenantconfig,
					Actions: []RepoAction{
						RepoActionFirst,
					},
				},
				{
					ID: RepoResourceTypeWorkflow,
					Actions: []RepoAction{
						RepoActionUpdate,
						RepoActionFirst,
					},
				},
				{
					ID: RepoResourceTypeWorkflowApprover,
					Actions: []RepoAction{
						RepoActionCreate,
						RepoActionDelete,
					},
				},
				{
					ID: RepoResourceTypeKey,
					Actions: []RepoAction{
						RepoActionFirst,
						// Update for setting the editable state
						RepoActionUpdate,
					},
				},
				{
					ID: RepoResourceTypeGroup,
					Actions: []RepoAction{
						RepoActionDynamic,
						RepoActionCount,
						RepoActionList,
					},
				},
				{
					ID: RepoResourceTypeKeyconfiguration,
					Actions: []RepoAction{
						RepoActionDynamic,
						RepoActionFirst,
						RepoActionList,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeSystem,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionList,
						RepoActionFirst,
					},
				},
				{
					ID: RepoResourceTypeSystemProperty,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionList,
					},
				},
				{
					ID: RepoResourceTypeEvent,
					Actions: []RepoAction{
						RepoActionFirst,
						RepoActionList,
						RepoActionCount,
					},
				},
			},
		),
		NewPolicy(
			"InternalTaskHYOKSync",
			constants.InternalTaskHYOKSyncRole,
			[]BaseResourceType[RepoResourceTypeName, RepoAction]{
				{
					ID: RepoResourceTypeKey,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionList,
						RepoActionUpdate,
					},
				},
			},
		),
		NewPolicy(
			"InternalTaskKeystorePool",
			constants.InternalTaskKeystorePoolRole,
			[]BaseResourceType[RepoResourceTypeName, RepoAction]{
				{
					ID: RepoResourceTypeKeystore,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionCreate,
					},
				},
			},
		),
		NewPolicy(
			"InternalTaskSystemRefresh",
			constants.InternalTaskSystemRefreshRole,
			[]BaseResourceType[RepoResourceTypeName, RepoAction]{
				{
					ID: RepoResourceTypeKeyconfiguration,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionCount,
					},
				},
				{
					ID: RepoResourceTypeSystem,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionList,
					},
				},
				{
					ID: RepoResourceTypeSystemProperty,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionList,
						RepoActionUpdate,
					},
				},
				{
					ID: RepoResourceTypeEvent,
					Actions: []RepoAction{
						RepoActionCount,
						RepoActionList,
					},
				},
			},
		),
		NewPolicy(
			"InternalTaskTenantRefresh",
			constants.InternalTaskTenantRefreshRole,
			[]BaseResourceType[RepoResourceTypeName, RepoAction]{
				{
					ID: RepoResourceTypeTenant,
					Actions: []RepoAction{
						RepoActionUpdate,
					},
				},
			},
		),
		NewPolicy(
			"InternalTaskWorkflowCleanup",
			constants.InternalTaskWorkflowCleanupRole,
			[]BaseResourceType[RepoResourceTypeName, RepoAction]{
				{
					ID: RepoResourceTypeWorkflow,
					Actions: []RepoAction{
						RepoActionList,
						RepoActionCount,
						RepoActionDelete,
					},
				},
			},
		),
		NewPolicy(
			"InternalTaskWorkExpiration",
			constants.InternalTaskWorkflowExpirationRole,
			[]BaseResourceType[RepoResourceTypeName, RepoAction]{
				{
					ID: RepoResourceTypeGroup,
					Actions: []RepoAction{
						RepoActionDynamic,
					},
				},
				{
					ID: RepoResourceTypeWorkflow,
					Actions: []RepoAction{
						RepoActionFirst,
						RepoActionList,
						RepoActionCount,
						RepoActionUpdate,
					},
				},
			},
		),
	},
}

func init() {
	// Index policies by role for fast lookup
	RepoInternalPolicies = make(map[constants.InternalRole][]BasePolicy[
		constants.InternalRole, RepoResourceTypeName, RepoAction])
	for _, policy := range InternalRepoPolicyData.Policies {
		RepoInternalPolicies[policy.Role] = append(RepoInternalPolicies[policy.Role], policy)
	}
}
