package authz

import (
	"github.com/openkcm/cmk/internal/constants"
)

var APIPolicies = RolePolicies[constants.BusinessRole, APIResourceType, APIAction]{
	constants.TenantAuditorRole: {
		{
			ID: constants.AuditorPolicy,
			ResourceTypes: []Resource[APIResourceType, APIAction]{
				{
					Type: APIResourceTypeKeyConfiguration,
					Actions: []APIAction{
						APIActionRead,
					},
				},
				{
					Type: APIResourceTypeKey,
					Actions: []APIAction{
						APIActionRead,
					},
				},
				{
					Type: APIResourceTypeSystem,
					Actions: []APIAction{
						APIActionRead,
					},
				},
				{
					Type: APIResourceTypeWorkFlow,
					Actions: []APIAction{
						APIActionRead,
					},
				},
				{
					Type: APIResourceTypeTenantSettings,
					Actions: []APIAction{
						APIActionRead,
					},
				},
				{
					Type: APIResourceTypeUserGroup,
					Actions: []APIAction{
						APIActionRead,
					},
				},
				{
					Type: APIResourceTypeTenant,
					Actions: []APIAction{
						APIActionRead,
					},
				},
			},
		},
	},
	constants.KeyAdminRole: {
		{
			ID: constants.KeyAdminPolicy,
			ResourceTypes: []Resource[APIResourceType, APIAction]{
				{
					Type: APIResourceTypeKeyConfiguration,
					Actions: []APIAction{
						APIActionRead,
						APIActionCreate,
						APIActionDelete,
						APIActionUpdate,
					},
				},
				{
					Type: APIResourceTypeKey,
					Actions: []APIAction{
						APIActionRead,
						APIActionCreate,
						APIActionDelete,
						APIActionUpdate,
						APIActionKeyRotate,
					},
				},
				{
					Type: APIResourceTypeUserGroup,
					Actions: []APIAction{
						APIActionRead,
					},
				},
				{
					Type: APIResourceTypeSystem,
					Actions: []APIAction{
						APIActionSystemModifyLink,
						APIActionRead,
						APIActionUpdate,
					},
				},
				{
					Type: APIResourceTypeWorkFlow,
					Actions: []APIAction{
						APIActionRead,
						APIActionCreate,
						APIActionDelete,
						APIActionUpdate,
					},
				},
				{
					Type: APIResourceTypeTenantSettings,
					Actions: []APIAction{
						APIActionRead,
					},
				},
			},
		},
	},
	constants.TenantAdminRole: {
		{
			ID: constants.TenantAdminPolicy,
			ResourceTypes: []Resource[APIResourceType, APIAction]{
				{
					Type: APIResourceTypeTenant,
					Actions: []APIAction{
						APIActionRead,
						APIActionUpdate,
					},
				},
				{
					Type: APIResourceTypeUserGroup,
					Actions: []APIAction{
						APIActionRead,
						APIActionCreate,
						APIActionDelete,
						APIActionUpdate,
					},
				},
				{
					Type: APIResourceTypeTenantSettings,
					Actions: []APIAction{
						APIActionRead,
						APIActionUpdate,
					},
				},
			},
		},
	},
}
