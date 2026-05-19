package authz

import (
	"github.com/openkcm/cmk/internal/constants"
)

var APIBusinessPolicies = make(map[constants.BusinessRole][]BasePolicy[
	constants.BusinessRole, APIResourceTypeName, APIAction])

type policies struct {
	Roles    []constants.BusinessRole
	Policies []BasePolicy[constants.BusinessRole, APIResourceTypeName, APIAction]
}

var PolicyData = policies{
	Roles: []constants.BusinessRole{
		constants.KeyAdminRole, constants.TenantAdminRole, constants.TenantAuditorRole,
	},
	Policies: []BasePolicy[constants.BusinessRole, APIResourceTypeName, APIAction]{
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
	APIBusinessPolicies = make(map[constants.BusinessRole][]BasePolicy[constants.BusinessRole,
		APIResourceTypeName, APIAction])
	for _, policy := range PolicyData.Policies {
		APIBusinessPolicies[policy.Role] = append(APIBusinessPolicies[policy.Role], policy)
	}
}
